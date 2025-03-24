package store

import (
	"fmt"
	"testing"
)

func TestAddTotpUrl(t *testing.T) {
	s := NewStore("/dev/null", "password")
	err := s.AddUrl("otpauth://totp/Example:alice@google.com?secret=JBSWY3DPEHPK3PXP")
	if err != nil {
		panic(err)
	}
	err = s.Save()
	if err != nil {
		panic(err)
	}
	got := fmt.Sprintf("%v %v %v %v", len(s.Entries), s.Entries[0].Issuer, s.Entries[0].Name, s.Entries[0].Secret)
	expected := "1 Example alice@google.com JBSWY3DPEHPK3PXP"
	if got != expected {
		t.Errorf("got %v, expected %v", got, expected)
	}
}

func TestAddMigrationUrl(t *testing.T) {
	s := NewStore("/dev/null", "password")
	err := s.AddUrl("otpauth-migration://offline?data=CjEKCkhlbGxvId6tvu8SGEV4YW1wbGU6YWxpY2VAZ29vZ2xlLmNvbRoHRXhhbXBsZTAC")
	if err != nil {
		panic(err)
	}
	err = s.Save()
	if err != nil {
		panic(err)
	}
	got := fmt.Sprintf("%v %v %v %v", len(s.Entries), s.Entries[0].Issuer, s.Entries[0].Name, s.Entries[0].Secret)
	expected := "1 Example alice@google.com JBSWY3DPEHPK3PXP"
	if got != expected {
		t.Errorf("got %v, expected %v", got, expected)
	}
}
