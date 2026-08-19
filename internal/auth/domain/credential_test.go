package domain

import "testing"

// 포털은 지금 OIDC 아니면 session 둘 중 하나만 쓴다. session 모드는 클라이언트가
// 보낸 X-User-* 헤더를 그대로 믿어 사실상 무인증이고, 비밀번호를 담을 곳도 없었다.
// IdP 가 죽어도 들어갈 수단이 있어야 하므로 실제 자격 검증을 둔다.

func TestHashPassword_VerifiesAgainstOriginal(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if hash == "correct-horse-battery" {
		t.Fatal("평문이 그대로 저장되면 안 된다")
	}
	if err := VerifyPassword(hash, "correct-horse-battery"); err != nil {
		t.Fatalf("올바른 비밀번호가 거부됐다: %v", err)
	}
}

func TestVerifyPassword_RejectsWrongPassword(t *testing.T) {
	hash, _ := HashPassword("correct-horse-battery")
	if err := VerifyPassword(hash, "wrong") == nil; err {
		t.Fatal("틀린 비밀번호가 통과했다")
	}
}

// 같은 평문이라도 매번 다른 해시가 나와야 한다(salt). 같은 해시가 나오면
// 유출 시 같은 비밀번호를 쓰는 계정이 한꺼번에 드러난다.
func TestHashPassword_IsSalted(t *testing.T) {
	a, _ := HashPassword("same-password")
	b, _ := HashPassword("same-password")
	if a == b {
		t.Fatal("같은 평문이 같은 해시를 냈다 — salt 가 없다")
	}
}

// 비밀번호가 설정되지 않은 계정(OIDC 전용)은 빈 해시를 갖는다. 빈 해시로
// 로그인이 통과하면 그 계정 전부가 무인증으로 열린다.
func TestVerifyPassword_EmptyHashNeverPasses(t *testing.T) {
	for _, attempt := range []string{"", "anything"} {
		if err := VerifyPassword("", attempt); err == nil {
			t.Fatalf("빈 해시가 %q 를 통과시켰다", attempt)
		}
	}
}

// 정책은 최소한이라도 있어야 한다 — 없으면 빈 비밀번호가 저장된다.
func TestValidatePasswordPolicy(t *testing.T) {
	if err := ValidatePasswordPolicy("short"); err == nil {
		t.Fatal("너무 짧은 비밀번호가 통과했다")
	}
	if err := ValidatePasswordPolicy("long-enough-password"); err != nil {
		t.Fatalf("정상 비밀번호가 거부됐다: %v", err)
	}
}
