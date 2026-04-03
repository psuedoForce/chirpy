package auth

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCreateJwt(t *testing.T) {
	uid := uuid.New()
	_, err := MakeJWT(uid, "testTokenSecret", time.Duration(500))
	if err != nil {
		fmt.Println(err)
		t.Errorf("Unable to create token")
	}
}

func TestValidateTokenPassed(t *testing.T) {
	uid := uuid.New()
	res, err := MakeJWT(uid, "testTokenSecret", time.Duration(5000))
	if err != nil {
		t.Errorf("Unable to create token")
	}
	token, eToken := ValidateJWT(res, "testTokenSecret")
	if eToken != nil {
		fmt.Println(eToken)
		t.Errorf("Unable to Validate Token")
	}
	if token != uid {
		t.Errorf("Token Validation failed")
	}
}

func TestValidateTokenFailed(t *testing.T) {
	uid := uuid.New()
	res, err := MakeJWT(uid, "testTokenSecret", time.Duration(5000))
	if err != nil {
		t.Errorf("Unable to create token")
	}
	time.Sleep(5000 * time.Millisecond)
	token, eToken := ValidateJWT(res, "testTokenSecret")
	if eToken == nil {
		fmt.Println(eToken)
		t.Errorf("Token expired but still no error")
	}
	if token == uid {
		t.Errorf("Token expired but still matched")
	}
}

func TestGetBearerToken(t *testing.T) {
	uid := uuid.New()
	res, err := MakeJWT(uid, "testTokenSecret", time.Duration(5000))
	if err != nil {
		t.Errorf("Unable to create token")
	}
	headers := http.Header{}
	token := fmt.Sprintf("Bearer %v", res)
	headers.Set("Autorization", token)
	bt, err := GetBearerToken(headers)
	if err != nil {
		fmt.Println((err))
		t.Errorf("Error when fetching bearer token")
	}
	if bt != res {
		t.Errorf("Unable to fetch bearer token")
	}
}
