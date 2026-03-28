package domain

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrAlertRuleNotFound_HasExpectedMessage(t *testing.T) {
	assert.EqualError(t, ErrAlertRuleNotFound, "alert rule not found")
}

func TestErrAlertNotFound_HasExpectedMessage(t *testing.T) {
	assert.EqualError(t, ErrAlertNotFound, "alert not found")
}

func TestDomainErrors_AreSentinelComparable(t *testing.T) {
	errRule := errors.Join(ErrAlertRuleNotFound, errors.New("wrapped"))
	errAlert := errors.Join(ErrAlertNotFound, errors.New("wrapped"))

	assert.True(t, errors.Is(errRule, ErrAlertRuleNotFound))
	assert.True(t, errors.Is(errAlert, ErrAlertNotFound))
}
