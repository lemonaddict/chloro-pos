package identity

import (
	"crypto/rand"
	"strings"
)

// Kasir codes use Crockford's base32 alphabet: no I, L, O or U, so a code copied from a sticky
// note is hard to misread. A code is 4 selector characters (find the seat) followed by 8 secret
// characters (40 bits), shown as SSSS-XXXX-XXXX.
const (
	codeAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	selectorLen  = 4
	secretLen    = 8
)

type cashierCode struct{ Selector, Secret string }

// String is the form shown to the owner once, when the code is made.
func (c cashierCode) String() string {
	return c.Selector + "-" + c.Secret[:4] + "-" + c.Secret[4:]
}

func randomChars(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = codeAlphabet[b[i]&31] // 32 symbols, so the mask keeps every one equally likely
	}
	return string(b), nil
}

func newCashierCode() (cashierCode, error) {
	sel, err := randomChars(selectorLen)
	if err != nil {
		return cashierCode{}, err
	}
	secret, err := randomChars(secretLen)
	if err != nil {
		return cashierCode{}, err
	}
	return cashierCode{Selector: sel, Secret: secret}, nil
}

// parseCashierCode accepts what a person types: any case, with or without dashes or spaces.
// O reads as 0 and I or L as 1, the usual slips, since those letters are not in the alphabet.
func parseCashierCode(raw string) (cashierCode, bool) {
	if len(raw) > 64 {
		return cashierCode{}, false
	}
	var b strings.Builder
	for _, r := range strings.ToUpper(raw) {
		switch r {
		case '-', ' ':
			continue
		case 'O':
			r = '0'
		case 'I', 'L':
			r = '1'
		}
		b.WriteRune(r)
	}
	s := b.String()
	if len(s) != selectorLen+secretLen {
		return cashierCode{}, false
	}
	for i := 0; i < len(s); i++ {
		if strings.IndexByte(codeAlphabet, s[i]) < 0 {
			return cashierCode{}, false
		}
	}
	return cashierCode{Selector: s[:selectorLen], Secret: s[selectorLen:]}, true
}
