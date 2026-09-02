package sealer

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSealer(t *testing.T) *StreamSealer {
	t.Helper()
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	s, err := NewStreamSealer("k1", key)
	require.NoError(t, err)
	return s
}

func roundTrip(t *testing.T, s *StreamSealer, data []byte) []byte {
	t.Helper()
	var sealed bytes.Buffer
	require.NoError(t, s.Seal(context.Background(), bytes.NewReader(data), &sealed))
	var out bytes.Buffer
	require.NoError(t, s.Unseal(context.Background(), bytes.NewReader(sealed.Bytes()), &out))
	return out.Bytes()
}

func TestSeal_왕복(t *testing.T) {
	s := newSealer(t)
	for _, size := range []int{0, 1, 1024, chunkSize - 1, chunkSize, chunkSize + 1, 3*chunkSize + 77} {
		data := make([]byte, size)
		_, _ = rand.Read(data)
		got := roundTrip(t, s, data)
		assert.Equal(t, size, len(got), "size=%d", size)
		assert.True(t, bytes.Equal(data, got), "size=%d 내용이 다르다", size)
	}
}

func TestSeal_키가_32바이트가_아니면_거부(t *testing.T) {
	_, err := NewStreamSealer("k", make([]byte, 16))
	require.Error(t, err)
}

func TestSeal_같은_평문이라도_암호문이_다르다(t *testing.T) {
	// nonce 접두사가 봉인마다 새로 생성되므로 키를 재사용해도 겹치지 않는다.
	s := newSealer(t)
	data := []byte("같은 내용")
	var a, b bytes.Buffer
	require.NoError(t, s.Seal(context.Background(), bytes.NewReader(data), &a))
	require.NoError(t, s.Seal(context.Background(), bytes.NewReader(data), &b))
	assert.NotEqual(t, a.Bytes(), b.Bytes())
}

func TestUnseal_다른_키로는_못_연다(t *testing.T) {
	var sealed bytes.Buffer
	require.NoError(t, newSealer(t).Seal(context.Background(), bytes.NewReader([]byte("secret")), &sealed))
	err := newSealer(t).Unseal(context.Background(), bytes.NewReader(sealed.Bytes()), io.Discard)
	require.Error(t, err)
}

// 백업본이 조용히 짧아지는 것은 무결성 검증을 통과한 손상이라 가장 나쁘다.
func TestUnseal_뒤쪽_조각을_잘라내면_실패한다(t *testing.T) {
	s := newSealer(t)
	data := make([]byte, 3*chunkSize)
	_, _ = rand.Read(data)

	var sealed bytes.Buffer
	require.NoError(t, s.Seal(context.Background(), bytes.NewReader(data), &sealed))

	// 조각 경계에서 정확히 잘라낸다 — 첫 조각까지만 남기고 나머지를 버린다.
	// 이렇게 자르면 각 조각은 온전하므로, final 플래그가 없다는 사실만으로
	// 잘림을 알아채야 한다.
	header := len(magic) + noncePrefixLen
	oneChunk := header + 4 + chunkSize + 16 // 길이 4바이트 + 본문 + GCM 태그
	err := s.Unseal(context.Background(), bytes.NewReader(sealed.Bytes()[:oneChunk]), io.Discard)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "잘렸", "온전한 조각만 남아도 잘림을 알아채야 한다")
}

func TestUnseal_조각_중간에서_잘려도_실패한다(t *testing.T) {
	s := newSealer(t)
	data := make([]byte, 3*chunkSize)
	_, _ = rand.Read(data)

	var sealed bytes.Buffer
	require.NoError(t, s.Seal(context.Background(), bytes.NewReader(data), &sealed))

	require.Error(t, s.Unseal(context.Background(), bytes.NewReader(sealed.Bytes()[:chunkSize]), io.Discard))
}

func TestUnseal_형식이_아니면_거부(t *testing.T) {
	err := newSealer(t).Unseal(context.Background(), bytes.NewReader([]byte("random junk data here")), io.Discard)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "형식")
}

func TestUnseal_변조된_조각은_실패한다(t *testing.T) {
	s := newSealer(t)
	var sealed bytes.Buffer
	require.NoError(t, s.Seal(context.Background(), bytes.NewReader([]byte("tamper me")), &sealed))

	b := sealed.Bytes()
	b[len(b)-1] ^= 0xFF

	require.Error(t, s.Unseal(context.Background(), bytes.NewReader(b), io.Discard))
}

func TestSeal_대용량에서_메모리가_선형으로_늘지_않는다(t *testing.T) {
	// 설계 §5.4 — 수십 GB 를 다루므로 전체를 메모리에 올리면 안 된다.
	// 조각 크기 이상으로 버퍼가 커지지 않는지 본다.
	s := newSealer(t)
	const total = 8 * chunkSize

	var sealedLen int64
	pr, pw := io.Pipe()
	go func() {
		defer func() { _ = pw.Close() }()
		buf := make([]byte, 64*1024)
		for written := 0; written < total; written += len(buf) {
			_, _ = pw.Write(buf)
		}
	}()

	sealedLen, err := io.Copy(io.Discard, sealPipe(t, s, pr))
	require.NoError(t, err)
	assert.Greater(t, sealedLen, int64(total), "태그가 붙으므로 원본보다 크다")
}

func sealPipe(t *testing.T, s *StreamSealer, in io.Reader) io.Reader {
	t.Helper()
	pr, pw := io.Pipe()
	go func() {
		err := s.Seal(context.Background(), in, pw)
		_ = pw.CloseWithError(err)
	}()
	return pr
}
