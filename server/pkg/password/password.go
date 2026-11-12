// Package password bcrypt 密码散列。
package password

import "golang.org/x/crypto/bcrypt"

// Hash 生成 bcrypt 散列(默认 cost)。
func Hash(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Verify 校验明文与散列是否匹配。
func Verify(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
