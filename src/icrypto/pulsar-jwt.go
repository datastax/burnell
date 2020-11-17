package icrypto

// This is JWT sign/verify with the same key algo used in Pulsar.

import (
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
)

// RSAKeyPair for JWT token sign and verification
type RSAKeyPair struct {
	PrivateKey           *rsa.PrivateKey
	PublicKey            *rsa.PublicKey
	PrivateKeyPKCS8Bytes []byte
	PublicKeyPKIXBytes   []byte
}

const (
	tokenDuration = 24
	expireOffset  = 3600
	bitSize       = 2048
)

var jwtRsaKeys *RSAKeyPair

// NewRSAKeyPair creates a pair of RSA key for JWT token sign and verification
func NewRSAKeyPair() (*RSAKeyPair, error) {
	reader := rand.Reader
	privateKey, err := rsa.GenerateKey(reader, bitSize)
	if err != nil {
		return nil, err
	}

	err = privateKey.Validate()
	if err != nil {
		return nil, err
	}

	return newRSAKeyPair(privateKey, &privateKey.PublicKey)
}

// LoadRSAKeyPair loads existing RSA key pair
func LoadRSAKeyPair(privateKeyPath, publicKeyPath string) (*RSAKeyPair, error) {
	privateKey, err := getPrivateKey(privateKeyPath)
	if err != nil {
		return nil, err
	}
	publicKey, err := getPublicKey(publicKeyPath)
	if err != nil {
		return nil, err
	}

	return newRSAKeyPair(privateKey, publicKey)
}

// LoadRSAKeyPairFromBase64 loads existing RSA key pair based on base64 []byte
func LoadRSAKeyPairFromBase64(privateKeyBase64, publicKeyBase64 []byte) (*RSAKeyPair, error) {
	privateKey, err := ParseX509PKCS8PrivateKey(privateKeyBase64)
	if err != nil {
		return nil, err
	}

	publicKey, err := ParseX509PKIXPublicKey(publicKeyBase64)
	if err != nil {
		return nil, err
	}
	return newRSAKeyPair(privateKey, publicKey)
}

func newRSAKeyPair(privateKey *rsa.PrivateKey, publicKey *rsa.PublicKey) (*RSAKeyPair, error) {
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, err
	}
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, err
	}
	// private key is valid at this point
	return &RSAKeyPair{
		PrivateKey:           privateKey,
		PublicKey:            publicKey,
		PrivateKeyPKCS8Bytes: privateKeyBytes,
		PublicKeyPKIXBytes:   publicKeyBytes,
	}, nil
}

// ExportRSAPublicKeyAsPEM exports RSA public key in PEM format as string
func (keys *RSAKeyPair) ExportRSAPublicKeyAsPEM() string {
	publicKeyPEM := pem.EncodeToMemory(
		&pem.Block{
			Bytes: keys.PublicKeyPKIXBytes,
		},
	)

	return string(publicKeyPEM)
}

// ExportRSAPrivateKeyAsPEM exports RSA private key in PEM format as string
func (keys *RSAKeyPair) ExportRSAPrivateKeyAsPEM() string {
	privateKeyPEM := pem.EncodeToMemory(
		&pem.Block{
			Bytes: keys.PrivateKeyPKCS8Bytes,
		},
	)

	return string(privateKeyPEM)
}

// ExportPrivateKeyBinaryBase64 exports RSA private key in binary as base64 format
func (keys *RSAKeyPair) ExportPrivateKeyBinaryBase64() string {
	return base64.StdEncoding.EncodeToString(keys.PrivateKeyPKCS8Bytes)
}

// ExportPublicKeyBinaryBase64 exports RSA public key in binary as base64 format
func (keys *RSAKeyPair) ExportPublicKeyBinaryBase64() string {
	return base64.StdEncoding.EncodeToString(keys.PublicKeyPKIXBytes)
}

// ExportRSAPublicKeyBinaryFile exports RSA public key PEM file
func (keys *RSAKeyPair) ExportRSAPublicKeyBinaryFile(filePath string) error {
	pemfile, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer pemfile.Close()

	err = binary.Write(pemfile, binary.LittleEndian, keys.PublicKeyPKIXBytes)
	if err != nil {
		return err
	}

	return nil
}

// ExportRSAPrivateKeyBinaryFile exports RSA private key PEM file
func (keys *RSAKeyPair) ExportRSAPrivateKeyBinaryFile(filePath string) error {
	pemfile, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer pemfile.Close()

	// Private key in PEM format
	err = binary.Write(pemfile, binary.LittleEndian, keys.PrivateKeyPKCS8Bytes)
	if err != nil {
		return err
	}

	return nil
}

// writePemToFile writes keys to a file
func writeKeyToFile(keyBytes []byte, saveFileTo string) error {
	err := ioutil.WriteFile(saveFileTo, keyBytes, 0666)
	if err != nil {
		return err
	}

	return nil
}

// GenerateToken generates token with user defined subject
func (keys *RSAKeyPair) GenerateToken(userSubject string) (string, error) {
	token := jwt.New(jwt.SigningMethodRS256)
	token.Claims = jwt.MapClaims{
		// "exp": time.Now().Add(time.Hour * time.Duration(24)).Unix(),
		// "iat": time.Now().Unix(),
		"sub": userSubject,
	}
	tokenString, err := token.SignedString(keys.PrivateKey)
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

// DecodeToken decodes a token string
func (keys *RSAKeyPair) DecodeToken(tokenStr string) (*jwt.Token, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		return keys.PublicKey, nil
	})

	if err != nil {
		return nil, err
	}

	if token.Valid {
		return token, nil
	}

	return nil, errors.New("invalid token")
}

//TODO: support multiple subjects in claims

// GetTokenSubject gets the subjects from a token
func (keys *RSAKeyPair) GetTokenSubject(tokenStr string) (string, error) {
	token, err := keys.DecodeToken(tokenStr)
	if err != nil {
		return "", err
	}
	claims := token.Claims.(jwt.MapClaims)
	subjects, ok := claims["sub"]
	if ok {
		return subjects.(string), nil
	}
	return "", errors.New("missing subjects")
}

// VerifyTokenSubject verifies a token string based on required matching subject
func (keys *RSAKeyPair) VerifyTokenSubject(tokenStr, subject string) (bool, error) {
	token, err := keys.DecodeToken(tokenStr)

	if err != nil {
		return false, err
	}

	claims := token.Claims.(jwt.MapClaims)

	if subject == claims["sub"] {
		return true, nil
	}

	return false, errors.New("incorrect sub")
}

// GetTokenRemainingValidity is the remaining seconds before token expires
func (keys *RSAKeyPair) GetTokenRemainingValidity(timestamp interface{}) int {
	if validity, ok := timestamp.(float64); ok {
		tm := time.Unix(int64(validity), 0)
		remainer := tm.Sub(time.Now())
		if remainer > 0 {
			return int(remainer.Seconds() + expireOffset)
		}
	}
	return expireOffset
}

// supports pk12 jks binary format
func readPK12(file string) ([]byte, error) {
	osFile, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	reader := bufio.NewReaderSize(osFile, 4)

	return ioutil.ReadAll(reader)
}

// decode PEM format to array of bytes
func decodePEM(pemFilePath string) ([]byte, error) {
	keyFile, err := os.Open(pemFilePath)
	defer keyFile.Close()
	if err != nil {
		return nil, err
	}

	pemfileinfo, _ := keyFile.Stat()
	pembytes := make([]byte, pemfileinfo.Size())

	buffer := bufio.NewReader(keyFile)
	_, err = buffer.Read(pembytes)

	data, _ := pem.Decode([]byte(pembytes))
	return data.Bytes, err
}

// ParseX509PKCS8PrivateKey creates rsa.PrivateKey based on byte data
func ParseX509PKCS8PrivateKey(data []byte) (*rsa.PrivateKey, error) {
	key, err := x509.ParsePKCS8PrivateKey(data)
	if err != nil {
		return nil, err
	}

	rsaPrivate, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("expected key to be of type *rsa.PrivateKey, but actual was %T", key)
	}

	return rsaPrivate, nil
}

// ParseX509PKIXPublicKey creates rsa.PublicKey based on byte data
func ParseX509PKIXPublicKey(data []byte) (*rsa.PublicKey, error) {
	publicKeyImported, err := x509.ParsePKIXPublicKey(data)
	if err != nil {
		return nil, err
	}

	rsaPub, ok := publicKeyImported.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("expected key to be of type *rsa.PublicKey, but actual was %T", publicKeyImported)
	}

	return rsaPub, nil
}

// Since we support PEM And binary fomat of PKCS12/X509 keys,
// this function tries to determine which format
func fileFormat(file string) (string, error) {
	osFile, err := os.Open(file)
	if err != nil {
		return "", err
	}
	reader := bufio.NewReaderSize(osFile, 4)
	// attempt to guess based on first 4 bytes of input
	data, err := reader.Peek(4)
	if err != nil {
		return "", err
	}

	magic := binary.BigEndian.Uint32(data)
	if magic == 0x2D2D2D2D || magic == 0x434f4e4e {
		// Starts with '----' or 'CONN' (what s_client prints...)
		return "PEM", nil
	}
	if magic&0xFFFF0000 == 0x30820000 {
		// Looks like the input is DER-encoded, so it's either PKCS12 or X.509.
		if magic&0x0000FF00 == 0x0300 {
			// Probably X.509
			return "DER", nil
		}
		return "PKCS12", nil
	}

	return "", errors.New("undermined format")
}

func getDataFromKeyFile(file string) ([]byte, error) {
	format, err := fileFormat(file)
	if err != nil {
		return nil, err
	}

	switch format {
	case "PEM":
		return decodePEM(file)
	case "PKCS12":
		fmt.Println("PKCS12")
		return readPK12(file)
	default:
		return nil, errors.New("unsupported format")
	}
}

func getPrivateKey(file string) (*rsa.PrivateKey, error) {
	data, err := getDataFromKeyFile(file)
	if err != nil {
		return nil, err
	}

	return ParseX509PKCS8PrivateKey(data)
}

func getPublicKey(file string) (*rsa.PublicKey, error) {
	data, err := getDataFromKeyFile(file)
	if err != nil {
		return nil, err
	}

	return ParseX509PKIXPublicKey(data)
}
