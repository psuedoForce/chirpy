package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/psuedoforce/chirpy/internal/auth"
	"github.com/psuedoforce/chirpy/internal/database"
)

type apiConfig struct {
	mu            sync.Mutex
	filserverHits atomic.Int32
	queries       *database.Queries
	secretToken   string
	polkaKey      string
}
type create_chirp_req struct {
	Body   string    `json:"body"`
	UserId uuid.UUID `json:"user_id"`
}
type chirp struct {
	Id        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserId    uuid.UUID `json:"user_id"`
}

func main() {
	godotenv.Load()
	dbUrl := os.Getenv("DB_URL")
	secretToken := os.Getenv("SECRET_TOKEN")
	polkaKey := os.Getenv("POLKA_KEY")
	db, err := sql.Open("postgres", dbUrl)
	if err != nil {
		fmt.Println(err)
	}
	cfg := apiConfig{}
	cfg.queries = database.New(db)
	cfg.secretToken = secretToken
	cfg.polkaKey = polkaKey
	servMux := http.NewServeMux()
	handler := http.StripPrefix("/app/", http.FileServer(http.Dir(".")))
	servMux.Handle("/app/", cfg.middlewareMetricsInc(handler))
	servMux.Handle("/api/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("./assets"))))
	servMux.HandleFunc("GET /api/healthz", handleHealth)
	servMux.HandleFunc("GET /admin/metrics", cfg.handleMetric)
	servMux.HandleFunc("POST /admin/reset", cfg.reset)
	servMux.HandleFunc("POST /api/users", cfg.handleCreateUser)
	servMux.HandleFunc("POST /api/chirps", cfg.handleCreateChirp)
	servMux.HandleFunc("POST /api/login", cfg.handleLogin)
	servMux.HandleFunc("GET /api/chirps", cfg.handleGetAllChirps)
	servMux.HandleFunc("GET /api/chirps/{chirpID}", cfg.handleGetChirp)
	servMux.HandleFunc("POST /api/refresh", cfg.handleRefresh)
	servMux.HandleFunc("POST /api/revoke", cfg.handleRevoke)
	servMux.HandleFunc("PUT /api/users", cfg.handleUpdateUser)
	servMux.HandleFunc("DELETE /api/chirps/{chirpID}", cfg.handleDeleteChirp)
	servMux.HandleFunc("POST /api/polka/webhooks", cfg.handlePolkaWebHook)

	server := http.Server{
		Handler: servMux,
		Addr:    ":8080",
	}

	e := server.ListenAndServe()
	if e != nil {
		return
	}
}

func (a *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.filserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte("OK"))
}

func (c *apiConfig) handleMetric(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	c.mu.Lock()
	defer c.mu.Unlock()
	hits := fmt.Sprintf(`<html>
	<body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %v times!</p>
  </body>
</html>`, c.filserverHits.Load())
	w.Write([]byte(hits))
}

func validateChirp(body string) (string, bool) {
	if len(body) > 140 {
		return body, false
	}

	strs := strings.Fields(body)
	for i, s := range strs {
		if strings.EqualFold(s, "kerfuffle") || strings.EqualFold(s, "sharbert") || strings.EqualFold(s, "fornax") {
			strs[i] = "****"
		}
	}
	result_str := strings.Join(strs, " ")
	return result_str, true
}

func (c *apiConfig) reset(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Handling Reset")
	res := os.Getenv("PLATFORM")
	if res != "dev" {
		w.WriteHeader(403)
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.filserverHits.Store(0)
	c.queries.DeleteUsers(r.Context())
}

func (c *apiConfig) handleLogin(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Handling Login")
	type loginUserReq struct {
		Email     string `json:"email"`
		Password  string `json:"password"`
		ExpiresIn int64  `json:"expires_in_seconds"`
	}
	type userRes struct {
		Id           uuid.UUID `json:"id"`
		Created_at   time.Time `json:"created_at"`
		Updated_at   time.Time `json:"updated_at"`
		Email        string    `json:"email"`
		Token        string    `json:"token"`
		RefreshToken string    `json:"refresh_token"`
		IsChirpyRed  bool      `json:"is_chirpy_red"`
	}
	req := loginUserReq{}
	decoder := json.NewDecoder(r.Body)
	eJs := decoder.Decode(&req)
	if eJs != nil {
		w.WriteHeader(401)
		return
	}
	user, eUser := c.queries.GetUsersByEmail(r.Context(), sql.NullString{
		String: req.Email,
		Valid:  true,
	})
	if eUser != nil {
		w.WriteHeader(401)
		return
	}
	resCheckHash, eCheckHash := auth.CheckPasswordHash(req.Password, user.HashedPassword)
	if eCheckHash != nil || !resCheckHash {
		w.WriteHeader(401)
		return
	}

	if req.ExpiresIn == 0 {
		req.ExpiresIn = 3600
	}

	jwt, eJwt := auth.MakeJWT(user.ID, c.secretToken, time.Duration(req.ExpiresIn))
	if eJwt != nil {
		return
	}
	refreshToken := auth.MakeRefreshToken()
	e_insert_refresh := c.queries.InsertRefreshToken(context.Background(), database.InsertRefreshTokenParams{
		Token: refreshToken,
		CreatedAt: sql.NullTime{
			Time:  time.Now(),
			Valid: true,
		},
		UpdatedAt: sql.NullTime{
			Time:  time.Now(),
			Valid: true,
		},
		UserID: user.ID,
		ExpiresAt: sql.NullTime{
			Time:  time.Now().AddDate(0, 0, 60),
			Valid: true,
		},
	})

	if e_insert_refresh != nil {
		w.WriteHeader(401)
		return
	}
	response := userRes{
		Id:           user.ID,
		Created_at:   user.CreatedAt,
		Updated_at:   user.UpdatedAt,
		Email:        user.Email.String,
		Token:        jwt,
		RefreshToken: refreshToken,
		IsChirpyRed:  user.IsChirpyRed.Bool,
	}
	responseJs, eJs := json.Marshal(response)
	if eJs != nil {
		w.WriteHeader(401)
		return
	}
	w.WriteHeader(200)
	w.Write(responseJs)
}

func (c *apiConfig) handleRefresh(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Handling Refresh")
	type tokenResponse struct {
		Token string `json:"token"`
	}

	tokenStr := r.Header.Get("Authorization")
	tokenList := strings.Fields(tokenStr)
	if len(tokenList) < 2 {
		w.WriteHeader(401)
		return
	}
	token := tokenList[1]
	t, e := c.queries.GetRreshToken(r.Context(), token)
	if e != nil {
		fmt.Println(e)
		w.WriteHeader(401)
		return
	}
	if t.RevokedAt.Valid {
		fmt.Println("Revoked at is Not Null")
		w.WriteHeader(401)
		return
	}
	if t.ExpiresAt.Time.Before(time.Now()) {
		fmt.Println("Token Expired")
		w.WriteHeader(401)
		return
	}
	user, e_user := c.queries.GetuserFromRefreshToken(r.Context(), token)
	if e_user != nil {
		fmt.Println(e_user)
		w.WriteHeader(401)
		return
	}
	uuid := user.ID
	acc_t, e_acc := auth.MakeJWT(uuid, c.secretToken, time.Duration(3600))
	if e_acc != nil {
		fmt.Println(e_acc)
		w.WriteHeader(401)
		return
	}
	response := tokenResponse{
		Token: acc_t,
	}
	mb, me := json.Marshal(response)
	if me != nil {
		fmt.Println(me)
		w.WriteHeader(401)
		return
	}
	w.Write(mb)
}

func (c *apiConfig) handleRevoke(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Handling Revoke")

	tokenStr := r.Header.Get("Authorization")
	tokenList := strings.Fields(tokenStr)
	if len(tokenList) < 2 {
		w.WriteHeader(401)
		return
	}
	token := tokenList[1]
	time := time.Now()
	t, e := c.queries.UpdateRefreshToken(r.Context(), database.UpdateRefreshTokenParams{
		UpdatedAt: sql.NullTime{
			Time:  time,
			Valid: true,
		},
		Token: token,
	})
	fmt.Printf(" Token %v, Updated at %v ; Revoked at %v", token, t.UpdatedAt, t.RevokedAt)
	if e != nil {
		w.WriteHeader(404)
		return
	}
	w.WriteHeader(204)
}

func (c *apiConfig) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Handling Create User")
	type createUserReq struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	type createUserRes struct {
		Id          uuid.UUID `json:"id"`
		Created_at  time.Time `json:"created_at"`
		Updated_at  time.Time `json:"updated_at"`
		Email       string    `json:"email"`
		IsChirpyRed bool      `json:"is_chirpy_red"`
	}
	req := createUserReq{}
	decoder := json.NewDecoder(r.Body)
	e := decoder.Decode(&req)
	if e != nil {
		fmt.Println(e)
		return
	}
	hashedPass, ePass := auth.HashPassword(req.Password)
	if ePass != nil {
		return
	}
	res, err := c.queries.CreateUser(r.Context(), database.CreateUserParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Email: sql.NullString{
			String: req.Email,
			Valid:  req.Email != "",
		},
		HashedPassword: hashedPass,
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	createUserResp := createUserRes{
		Id:          res.ID,
		Created_at:  res.CreatedAt,
		Updated_at:  res.UpdatedAt,
		Email:       res.Email.String,
		IsChirpyRed: res.IsChirpyRed.Bool,
	}
	fmt.Println("Printing Response")
	fmt.Println(createUserResp)
	mr, me := json.Marshal(createUserResp)
	if me != nil {
		fmt.Println(me)
		return
	}
	w.WriteHeader(201)
	w.Write(mr)
}

func (c *apiConfig) handleCreateChirp(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Handling Create Chirp")
	jwt, eJwt := auth.GetBearerToken(r.Header)
	if eJwt != nil {
		fmt.Println(eJwt)
		w.WriteHeader(401)
		return
	}
	uid, euid := auth.ValidateJWT(jwt, c.secretToken)
	if euid != nil {
		fmt.Println(euid)
		w.WriteHeader(401)
		return
	}
	req_body := create_chirp_req{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&req_body)
	if err != nil {
		w.WriteHeader(400)
		w.Header().Add("Content-Type", "application/json")
		resp := chirp{}
		js, err := json.Marshal(resp)
		if err != nil {
			return
		}
		w.Write(js)
		return
	}
	res, err := c.queries.CreateChirp(r.Context(), database.CreateChirpParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Body:      req_body.Body,
		UserID:    uid,
	})

	if err != nil {
		return
	}

	js := chirp{
		Id:        res.ID,
		CreatedAt: res.CreatedAt,
		UpdatedAt: res.UpdatedAt,
		Body:      res.Body,
		UserId:    res.UserID,
	}
	response, e := json.Marshal(js)
	if e != nil {
		return
	}
	w.WriteHeader(201)
	w.Write(response)
}

func (c *apiConfig) handleGetAllChirps(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Handling Get ALL Chirps")
	queries := r.URL.Query()
	author := queries.Get("author_id")
	sort := queries.Get("sort")

	var getAllChirps []chirp
	var res []database.Chirp
	var err error
	if author != "" {
		authorId, eAuthor := uuid.Parse(author)
		if eAuthor != nil {
			w.WriteHeader(300)
			return
		}
		switch sort {
		case "asc", "":
			res, err = c.queries.GetAllChirpsForUserASC(r.Context(), authorId)
		case "desc":
			res, err = c.queries.GetAllChirpsForUserDESC(r.Context(), authorId)
		}

	} else {
		switch sort {
		case "asc", "":
			res, err = c.queries.GetAllChirpsASC(r.Context())
		case "desc":
			res, err = c.queries.GetAllChirpsDESC(r.Context())
		}
	}
	if err != nil {
		return
	}
	for _, item := range res {
		getAllChirps = append(getAllChirps, chirp{
			Id:        item.ID,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
			Body:      item.Body,
			UserId:    item.UserID,
		})
	}
	js, err := json.Marshal(getAllChirps)
	if err != nil {
		w.WriteHeader(300)
		return
	}
	w.WriteHeader(200)
	w.Write(js)
}

func (c *apiConfig) handleGetChirp(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Handling Get Chirp")
	chirpId, e := uuid.Parse(r.PathValue("chirpID"))
	if e != nil {
		return
	}
	var getChirp chirp
	res, err := c.queries.GetChirp(r.Context(), chirpId)
	if err != nil {
		w.WriteHeader(404)
		return
	}
	getChirp = chirp{
		Id:        res.ID,
		CreatedAt: res.CreatedAt,
		UpdatedAt: res.UpdatedAt,
		Body:      res.Body,
		UserId:    res.UserID,
	}
	js, err := json.Marshal(getChirp)
	if err != nil {
		w.WriteHeader(300)
		return
	}
	w.WriteHeader(200)
	w.Write(js)
}

func (c *apiConfig) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Handling Create User")
	type updateUserReq struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	type updateUserRes struct {
		Id          uuid.UUID `json:"id"`
		Created_at  time.Time `json:"created_at"`
		Updated_at  time.Time `json:"updated_at"`
		Email       string    `json:"email"`
		IsChirpyRed bool      `json:"is_chirpy_red"`
	}
	authString := r.Header.Get("Authorization")
	fieldStr := strings.Fields(authString)
	if len(fieldStr) < 2 {
		w.WriteHeader(401)
		return
	}
	req := updateUserReq{}
	decoder := json.NewDecoder(r.Body)
	e := decoder.Decode(&req)
	if e != nil {
		fmt.Println(e)
		return
	}
	hashedPass, ePass := auth.HashPassword(req.Password)
	if ePass != nil {
		return
	}
	uid, eVToken := auth.ValidateJWT(fieldStr[1], c.secretToken)
	if eVToken != nil {
		w.WriteHeader(401)
		return
	}

	user, eUser := c.queries.UpdateUserEmailPass(r.Context(), database.UpdateUserEmailPassParams{
		Email: sql.NullString{
			String: req.Email,
			Valid:  true,
		},
		UpdatedAt:      time.Now(),
		HashedPassword: hashedPass,
		ID:             uid,
	})
	if eUser != nil {
		w.WriteHeader(404)
		return
	}
	response := updateUserRes{
		Id:          user.ID,
		Created_at:  user.CreatedAt,
		Updated_at:  user.UpdatedAt,
		Email:       user.Email.String,
		IsChirpyRed: user.IsChirpyRed.Bool,
	}
	resJs, eJs := json.Marshal(response)
	if eJs != nil {
		w.WriteHeader(401)
		return
	}

	w.WriteHeader(200)
	w.Write(resJs)
}

func (c *apiConfig) handleDeleteChirp(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Handlign Delete User")
	ts, et := extractToken(r.Header)
	if et != nil {
		fmt.Println("Retrieving JWT error")
		fmt.Println(et)
		w.WriteHeader(401)
		return
	}
	uId, eValidate := auth.ValidateJWT(*ts, c.secretToken)
	if eValidate != nil {
		fmt.Println("Validate JWT token Error")
		fmt.Println(eValidate)
		w.WriteHeader(403)
		return
	}

	chirpId := r.PathValue("chirpID")
	cUid, eUid := uuid.Parse(chirpId)
	if eUid != nil {
		fmt.Println("User ID Error")
		fmt.Println(eUid)
		w.WriteHeader(403)
		return
	}
	chirp, eChirp := c.queries.GetChirp(r.Context(), cUid)
	if eChirp != nil {
		fmt.Println("Get Chirp Error")
		fmt.Println(eUid)
		w.WriteHeader(404)
		return
	}
	if uId != chirp.UserID {
		fmt.Println("User Does not equals chirp ID")
		fmt.Println(eUid)
		w.WriteHeader(403)
		return
	}
	eDelete := c.queries.DeleteChirp(r.Context(), cUid)
	if eDelete != nil {
		fmt.Println("Deletion error")
		fmt.Println(eUid)
		w.WriteHeader(404)
		return
	}
	w.WriteHeader(204)
}

func extractToken(header http.Header) (*string, error) {
	tokenStr := header.Get("Authorization")
	tokenList := strings.Fields(tokenStr)
	if len(tokenList) < 2 {
		return nil, fmt.Errorf("No Token Found")
	}
	token := tokenList[1]
	return &token, nil
}

func (c *apiConfig) handlePolkaWebHook(w http.ResponseWriter, r *http.Request) {
	key, eKey := auth.GetAPIKey(r.Header)
	if eKey != nil {
		w.WriteHeader(401)
		return
	}
	if key != c.polkaKey {
		w.WriteHeader(401)
		return
	}
	type polkaRequest struct {
		Event string `json:"event"`
		Data  struct {
			UserID string `json:"user_id"`
		} `json:"data"`
	}
	polkaReq := polkaRequest{}
	decoder := json.NewDecoder(r.Body)
	eDecode := decoder.Decode(&polkaReq)
	if eDecode != nil {
		w.WriteHeader(204)
		return
	}
	if polkaReq.Event != "user.upgraded" {
		w.WriteHeader(204)
		return
	}
	uuid, eUUID := uuid.Parse(polkaReq.Data.UserID)
	if eUUID != nil {
		w.WriteHeader(204)
		return
	}
	u, eU := c.queries.UpdateToChirpyRed(r.Context(), uuid)
	if eU != nil {
		fmt.Println("Error in updating to Chirpy red")
		fmt.Println(eU)
		w.WriteHeader(404)
		return
	}
	fmt.Printf("Updated the user %v\n", uuid)
	fmt.Println(u)
	w.WriteHeader(204)
}
