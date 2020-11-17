package tests

import (
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	. "github.com/kafkaesque-io/burnell/src/icrypto"
)

func TestRSAKeyPair(t *testing.T) {
	authen, err := NewRSAKeyPair()
	errNil(t, err)

	// authen := LoadRSAKeyPair(privKey, publicKey)
	tokenString, err := authen.GenerateToken("myadmin")
	errNil(t, err)
	fmt.Printf("%s", tokenString)

	token, err := authen.DecodeToken(tokenString)
	errNil(t, err)
	assert(t, token.Valid, "validate a valid token")

	privateKeyPath := "/tmp/unitest-keypair-private.key"
	err = authen.ExportRSAPrivateKeyBinaryFile(privateKeyPath)
	errNil(t, err)

	publicKeyPath := "/tmp/unitest-keypair-public.key"
	err = authen.ExportRSAPublicKeyBinaryFile(publicKeyPath)
	errNil(t, err)

	keyPairAuth, err := LoadRSAKeyPair(privateKeyPath, publicKeyPath)
	errNil(t, err)
	tokenString2, err := keyPairAuth.GenerateToken("myadmin")
	errNil(t, err)
	assert(t, tokenString == tokenString2, "two tokens must be identical")

	// save token to a file for pulsar cli validation
	err = ioutil.WriteFile("/tmp/myadmin.jwt", []byte(tokenString2), 0644)
	errNil(t, err)
	// pulsar token command to validate these TLS keys
	// docker run -it -v /tmp:/tmp apachepulsar/pulsar:2.6.1 bin/pulsar token create --secret-key /tmp/unitest-keypair-private.key --subject test-user

	// show token
	// docker run -it -v /tmp:/tmp apachepulsar/pulsar:2.6.1 bin/pulsar tokens show -f /tmp/myadmin.jwt

	// to validate a generated token with a public key
	// docker run -it -v /tmp:/tmp apachepulsar/pulsar:2.6.1 bin/pulsar tokens validate -pk /tmp/unitest-keypair-public.key -f /tmp/myadmin.jwt
}

func TestJWTRSASignAndVerifyWithPEMKey(t *testing.T) {
	privateKeyPath := "./example_private_key"
	publicKeyPath := "./example_public_key.pub"
	authen, err := LoadRSAKeyPair(privateKeyPath, publicKeyPath)
	errNil(t, err)

	tokenString, err := authen.GenerateToken("millet")
	errNil(t, err)
	assert(t, len(tokenString) > 1, "a token string can be generated")

	token, err0 := authen.DecodeToken(tokenString)
	errNil(t, err0)
	assert(t, token.Valid, "validate a valid token")

	valid, _ := authen.VerifyTokenSubject("bogustokenstr", "myadmin")
	assert(t, valid == false, "validate token fails test")

	valid, _ = authen.VerifyTokenSubject(tokenString, "millet")
	assert(t, valid, "validate token's expected subject")

	valid, _ = authen.VerifyTokenSubject(tokenString, "admin")
	assert(t, valid == false, "validate token's mismatched subject")

	pulsarGeneratedToken := "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJwaWNhc3NvIn0.TZilYXJOeeCLwNOHICCYyFxUlwOLxa_kzVjKcoQRTJm2xqmNzTn-s9zjbuaNMCDj1U7gRPHKHkWNDb2W4MwQd6Nkc543E_cIHlJG82eKKIsGfAEQpnPJLpzz2zytgmRON6HCPDsQDAKIXHriKmbmCzHLOILziks0oOCadBGC79iddb9DjPku6sU0nByS8r8_oIrRCqV_cNsH1MInA6CRNYkPJaJI0T8i77ND7azTXwH0FTX_KE_yRmOkXnejJ14GEEcBM99dPGg8jCp-zOyfvrMIJjWsWzjXYExxjKaC85779ciu59YO3cXd0Lk2LzlyB4kDKZgPyqOgyQFIfQ1eiA" // pragma: allowlist secret
	valid, err = authen.VerifyTokenSubject(pulsarGeneratedToken, "picasso")
	errNil(t, err)
	assert(t, valid, "validate pulsar generated token and subject")

	subjects, err := authen.GetTokenSubject(pulsarGeneratedToken)
	errNil(t, err)
	equals(t, subjects, "picasso")

	t2 := time.Now().Add(time.Hour * 1)
	expireOffset := authen.GetTokenRemainingValidity(t2)
	equals(t, expireOffset, 3600)

}
