// Package sealer 는 산출물 암호화 구현체다.
//
// 설계: docs/11_기능설계/Nullus_백업복구_설계.md §5.4 (nullus-plan#75)
//
// pkg/crypto/aes_gcm.go 를 쓰지 않는 이유: 그 API 는 []byte 전체를 받아
// base64 문자열을 돌려주므로 수십 GB 짜리 볼륨 아카이브에 쓸 수 없다.
// 같은 알고리즘(AES-256-GCM)의 스트리밍 변형이 필요하다.
package sealer

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	// chunkSize 는 한 번에 봉인하는 평문 크기다. GCM 은 스트림 암호가 아니라
	// 조각마다 태그가 붙으므로, 조각을 너무 작게 잡으면 태그 오버헤드가 커지고
	// 너무 크게 잡으면 메모리를 그만큼 쓴다.
	chunkSize = 1 << 20 // 1 MiB

	magic          = "NBK1" // Nullus BacKup v1
	noncePrefixLen = 8
	counterLen     = 4
)

// StreamSealer 는 AES-256-GCM 을 조각 단위로 적용한다.
//
// 조각마다 nonce 는 (랜덤 8바이트 접두사 || 조각 번호 4바이트) 다. 접두사가
// 봉인마다 새로 생성되므로 키 재사용에도 nonce 가 겹치지 않는다.
//
// 각 조각의 AAD 에 조각 번호와 "마지막 조각인가" 를 넣는다. 이것이 없으면
// 공격자가 뒤쪽 조각을 잘라내도 복호화가 성공해 버린다 — 백업본이 조용히
// 짧아지는 것은 무결성 검증을 통과한 손상이라 가장 나쁘다.
type StreamSealer struct {
	keyID string
	key   []byte
}

func NewStreamSealer(keyID string, key []byte) (*StreamSealer, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("백업 암호화 키는 32바이트여야 합니다 (현재 %d)", len(key))
	}
	return &StreamSealer{keyID: keyID, key: key}, nil
}

func (s *StreamSealer) KeyID() string { return s.keyID }

func (s *StreamSealer) gcm() (cipher.AEAD, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func aad(index uint32, final bool) []byte {
	b := make([]byte, 5)
	binary.BigEndian.PutUint32(b, index)
	if final {
		b[4] = 1
	}
	return b
}

func (s *StreamSealer) Seal(ctx context.Context, plaintext io.Reader, out io.Writer) error {
	gcm, err := s.gcm()
	if err != nil {
		return err
	}

	prefix := make([]byte, noncePrefixLen)
	if _, err := rand.Read(prefix); err != nil {
		return fmt.Errorf("nonce 접두사 생성: %w", err)
	}
	if _, err := out.Write([]byte(magic)); err != nil {
		return err
	}
	if _, err := out.Write(prefix); err != nil {
		return err
	}

	nonce := make([]byte, noncePrefixLen+counterLen)
	copy(nonce, prefix)

	buf := make([]byte, chunkSize)
	var index uint32
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, readErr := io.ReadFull(plaintext, buf)
		final := readErr == io.EOF || readErr == io.ErrUnexpectedEOF
		if readErr != nil && !final {
			return readErr
		}

		binary.BigEndian.PutUint32(nonce[noncePrefixLen:], index)
		sealed := gcm.Seal(nil, nonce, buf[:n], aad(index, final))

		var lenBuf [4]byte
		binary.BigEndian.PutUint32(lenBuf[:], uint32(len(sealed)))
		if _, err := out.Write(lenBuf[:]); err != nil {
			return err
		}
		if _, err := out.Write(sealed); err != nil {
			return err
		}
		if final {
			return nil
		}
		index++
	}
}

func (s *StreamSealer) Unseal(ctx context.Context, ciphertext io.Reader, out io.Writer) error {
	gcm, err := s.gcm()
	if err != nil {
		return err
	}

	header := make([]byte, len(magic)+noncePrefixLen)
	if _, err := io.ReadFull(ciphertext, header); err != nil {
		return fmt.Errorf("암호문 헤더를 읽을 수 없습니다: %w", err)
	}
	if string(header[:len(magic)]) != magic {
		return errors.New("백업 산출물 형식이 아닙니다")
	}

	nonce := make([]byte, noncePrefixLen+counterLen)
	copy(nonce, header[len(magic):])

	var index uint32
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		var lenBuf [4]byte
		if _, err := io.ReadFull(ciphertext, lenBuf[:]); err != nil {
			// 마지막 조각의 final 플래그를 보기 전에 스트림이 끝났다.
			return errors.New("암호문이 잘렸습니다: 마지막 조각을 찾지 못했습니다")
		}
		size := binary.BigEndian.Uint32(lenBuf[:])
		if size > chunkSize+uint32(gcm.Overhead()) {
			return fmt.Errorf("조각 크기가 비정상입니다: %d", size)
		}
		chunk := make([]byte, size)
		if _, err := io.ReadFull(ciphertext, chunk); err != nil {
			return fmt.Errorf("조각을 읽을 수 없습니다: %w", err)
		}

		binary.BigEndian.PutUint32(nonce[noncePrefixLen:], index)

		// 중간 조각으로 먼저 시도하고, 실패하면 마지막 조각으로 시도한다.
		plain, err := gcm.Open(nil, nonce, chunk, aad(index, false))
		if err != nil {
			plain, err = gcm.Open(nil, nonce, chunk, aad(index, true))
			if err != nil {
				return fmt.Errorf("조각 %d 복호화 실패 (키가 다르거나 손상됨)", index)
			}
			if _, werr := out.Write(plain); werr != nil {
				return werr
			}
			return nil
		}
		if _, werr := out.Write(plain); werr != nil {
			return werr
		}
		index++
	}
}
