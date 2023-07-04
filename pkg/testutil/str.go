package testutil

import (
	"math/rand"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

var (
	LettersForId        = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_-0123456789")
	LettersInvalidForId = []rune("~!@#$%^&*()=,./?'\"\\;:<>[]{}`")
)

func RandStr(n int, letterSetA []rune, LetterSetB []rune, justLetterSetA bool) string {
	b := make([]rune, n)
	hasLetterB := false
	for i := range b {
		vChance := 1
		if !justLetterSetA {
			vChance = 2
		}
		wantA := rand.Intn(vChance) == 0
		if wantA {
			b[i] = letterSetA[rand.Intn(len(letterSetA))]
		} else {
			hasLetterB = true
			b[i] = LetterSetB[rand.Intn(len(LetterSetB))]
		}
	}
	if !justLetterSetA && !hasLetterB {
		// make sure have one letter from B
		b[len(b)-1] = LetterSetB[rand.Intn(len(LetterSetB))]
	}
	return string(b)
}
