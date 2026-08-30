package device

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLocationID(t *testing.T) {
	var nilID LocationID
	assert.True(t, nilID.IsNil())
	assert.Equal(t, strings.Repeat("0", 32), nilID.String())

	id := LocationID{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef, 0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10}
	assert.False(t, id.IsNil())
	assert.Equal(t, "0123456789abcdeffedcba9876543210", id.String())
}

func TestGroupID(t *testing.T) {
	var nilID GroupID
	assert.True(t, nilID.IsNil())
	assert.Equal(t, strings.Repeat("0", 32), nilID.String())

	id := GroupID{0xff, 0xee, 0xdd, 0xcc, 0xbb, 0xaa, 0x99, 0x88, 0x77, 0x66, 0x55, 0x44, 0x33, 0x22, 0x11, 0x00}
	assert.False(t, id.IsNil())
	assert.Equal(t, "ffeeddccbbaa99887766554433221100", id.String())
}
