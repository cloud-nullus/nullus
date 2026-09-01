package bridge

import "context"

// pausable 은 회전 스케줄러가 제공하는 정지/재개 창구다.
type pausable interface {
	Pause()
	Resume()
}

// RotationPauser 는 백업이 정지 창을 여는 동안 토큰 회전을 멈춘다.
//
// 회전은 DB 와 금고를 함께 고쳐쓰므로, 멈추지 않으면 두 산출물 사이에
// 시점 어긋남이 생긴다 (설계 §2.1).
type RotationPauser struct {
	target pausable
}

func NewRotationPauser(target pausable) *RotationPauser {
	return &RotationPauser{target: target}
}

func (p *RotationPauser) Pause(context.Context) error {
	if p == nil || p.target == nil {
		return nil
	}
	p.target.Pause()
	return nil
}

func (p *RotationPauser) Resume(context.Context) error {
	if p == nil || p.target == nil {
		return nil
	}
	p.target.Resume()
	return nil
}
