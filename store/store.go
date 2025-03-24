package store

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/dim13/otpauth/migration"
	"github.com/mevdschee/fyne-authenticator/aescrypt"
)

type TotpStore struct {
	FileName string
	Password string
	Entries  []TotpEntry
}

type TotpEntry struct {
	Issuer string `json:"issuer"`
	Name   string `json:"name"`
	Secret string `json:"secret"`
}

func NewStore(filename, password string) *TotpStore {
	return &TotpStore{FileName: filename, Password: password}
}

func (s *TotpStore) Load() error {
	b, err := os.ReadFile(s.FileName)
	if err != nil {
		if os.IsNotExist(err) {
			b = []byte("[]")
		} else {
			return err
		}
	}
	str := string(b)
	if str != "[]" && str[:2] != "[{" {
		str, err = aescrypt.DecryptString(str, s.Password)
		if err != nil {
			return err
		}
	}
	err = json.Unmarshal([]byte(str), &s.Entries)
	if err != nil {
		return err
	}
	return nil
}

func (s *TotpStore) Save() error {
	b, err := json.Marshal(s.Entries)
	if err != nil {
		return err
	}
	str := string(b)
	str, err = aescrypt.EncryptString(str, s.Password)
	if err != nil {
		return err
	}
	// create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(s.FileName), 0755); err != nil {
		return err
	}
	return os.WriteFile(s.FileName, []byte(str), 0644)
}

func (s *TotpStore) addMigrationUrl(urlStr string) error {
	p, err := migration.UnmarshalURL(urlStr)
	if err != nil {
		return err
	}
	for _, op := range p.OtpParameters {
		name := op.GetName()
		nameParts := strings.SplitN(name, ":", 2)
		if len(nameParts) == 2 {
			op.Issuer = nameParts[0]
			op.Name = nameParts[1]
		}
		e := TotpEntry{Issuer: op.GetIssuer(), Name: op.GetName(), Secret: op.SecretString()}
		s.Entries = append(s.Entries, e)
	}
	return nil
}

func (s *TotpStore) addOtpUrl(host string, path string, queryParams url.Values) error {
	protocol := host
	if protocol != "totp" {
		return errors.New("protocol not supported")
	}
	pathParts := strings.SplitN(strings.TrimLeft(path, "/"), ":", 2)
	issuer := queryParams.Get("issuer")
	name := pathParts[0]
	if len(pathParts) == 2 {
		issuer = pathParts[0]
		name = pathParts[1]
	}
	secret := queryParams.Get("secret")
	if secret == "" {
		return errors.New("missing secret")
	}
	e := TotpEntry{Issuer: issuer, Name: name, Secret: secret}
	s.Entries = append(s.Entries, e)
	return nil
}

func (s *TotpStore) AddUrl(urlStr string) error {
	parsedUrl, err := url.Parse(urlStr)
	if err != nil {
		return err
	}
	scheme := parsedUrl.Scheme
	host := parsedUrl.Host
	path := parsedUrl.Path
	queryParams := parsedUrl.Query()
	switch scheme {
	case "otpauth":
		return s.addOtpUrl(host, path, queryParams)
	case "otpauth-migration":
		return s.addMigrationUrl(urlStr)
	}
	return errors.New("invalid url scheme")
}
