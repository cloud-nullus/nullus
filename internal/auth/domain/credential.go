// Package domain 은 인증 바운디드 컨텍스트의 도메인 규칙을 담는다.
package domain

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// ErrInvalidCredentials 는 이메일과 비밀번호가 맞지 않을 때다.
//
// 어느 쪽이 틀렸는지 구분해 알리지 않는다 — 구분하면 어떤 이메일이 가입돼 있는지
// 알아낼 수 있다.
var ErrInvalidCredentials = errors.New("invalid credentials")

// minPasswordLength 는 최소 길이다. 정책이 없으면 빈 비밀번호가 저장된다.
const minPasswordLength = 12

// HashPassword 는 저장용 bcrypt 해시를 만든다. 같은 평문이라도 매번 다른 값이
// 나온다(salt) — 같은 해시가 나오면 유출 시 같은 비밀번호를 쓰는 계정이 한꺼번에
// 드러난다.
func HashPassword(plaintext string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hashed), nil
}

// VerifyPassword 는 저장된 해시와 입력 평문을 대조한다.
//
// 해시가 비어 있으면 무엇도 통과시키지 않는다. 비밀번호를 설정하지 않은 계정
// (OIDC 전용)이 빈 해시를 갖는데, 여기서 통과시키면 그 계정 전부가 무인증으로
// 열린다.
func VerifyPassword(hash, plaintext string) error {
	if strings.TrimSpace(hash) == "" {
		return ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext)); err != nil {
		return ErrInvalidCredentials
	}
	return nil
}

// ValidatePasswordPolicy 는 저장 전에 최소 요건을 확인한다.
func ValidatePasswordPolicy(plaintext string) error {
	if len(plaintext) < minPasswordLength {
		return fmt.Errorf("비밀번호는 최소 %d자여야 합니다", minPasswordLength)
	}
	return nil
}
