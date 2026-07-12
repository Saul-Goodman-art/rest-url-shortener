package random

import (
	"math/rand"
	"time"
)

// NewRandomString generates random string with given size. Может содерж только буквы или только цифры
/*
Строка может быть:

	✅ Только буквы (например, "AbCdEf")

	✅ Только цифры (например, "123456")

	✅ Только заглавные (например, "ABCDEF")

	✅ Только строчные (например, "abcdef")

	✅ Смешанные (например, "A1b2C3") — это наиболее вероятно
*/
func NewRandomString(size int) string {
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

	chars := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"abcdefghijklmnopqrstuvwxyz" +
		"0123456789")

	b := make([]rune, size)
	for i := range b {
		b[i] = chars[rnd.Intn(len(chars))]
	}

	return string(b)
}
