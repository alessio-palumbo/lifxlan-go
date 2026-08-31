package device

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocationID(t *testing.T) {
	var nilID LocationID
	assert.True(t, nilID.IsNil())
	assert.Equal(t, "00000000-0000-0000-0000-000000000000", nilID.String())

	id := LocationID{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0x4d, 0xef, 0x8e, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10}
	assert.False(t, id.IsNil())
	assert.Equal(t, "01234567-89ab-4def-8edc-ba9876543210", id.String())
}

func TestGroupID(t *testing.T) {
	var nilID GroupID
	assert.True(t, nilID.IsNil())
	assert.Equal(t, "00000000-0000-0000-0000-000000000000", nilID.String())

	id := GroupID{0xff, 0xee, 0xdd, 0xcc, 0xbb, 0xaa, 0x49, 0x88, 0xb7, 0x66, 0x55, 0x44, 0x33, 0x22, 0x11, 0x00}
	assert.False(t, id.IsNil())
	assert.Equal(t, "ffeeddcc-bbaa-4988-b766-554433221100", id.String())
}

func TestParseLocationID(t *testing.T) {
	want := LocationID{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0x4d, 0xef, 0x8e, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10}
	for name, value := range map[string]string{
		"canonical":       "01234567-89ab-4def-8edc-ba9876543210",
		"compact LIFX":    "0123456789ab4def8edcba9876543210",
		"uppercase input": "01234567-89AB-4DEF-8EDC-BA9876543210",
	} {
		t.Run(name, func(t *testing.T) {
			got, err := ParseLocationID(value)
			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}
}

func TestParseGroupID(t *testing.T) {
	want := GroupID{0xff, 0xee, 0xdd, 0xcc, 0xbb, 0xaa, 0x49, 0x88, 0xb7, 0x66, 0x55, 0x44, 0x33, 0x22, 0x11, 0x00}
	for name, value := range map[string]string{
		"canonical":    "ffeeddcc-bbaa-4988-b766-554433221100",
		"compact LIFX": "ffeeddccbbaa4988b766554433221100",
	} {
		t.Run(name, func(t *testing.T) {
			got, err := ParseGroupID(value)
			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}
}

func TestParseIDRejectsInvalidUUIDs(t *testing.T) {
	for name, value := range map[string]string{
		"empty":           "",
		"wrong length":    "01234567-89ab-4def-8edc-ba987654321",
		"wrong separator": "01234567_89ab-4def-8edc-ba9876543210",
		"invalid hex":     "01234567-89ab-4def-8edc-ba987654321g",
	} {
		t.Run(name, func(t *testing.T) {
			_, locationErr := ParseLocationID(value)
			_, groupErr := ParseGroupID(value)
			assert.Error(t, locationErr)
			assert.Error(t, groupErr)
		})
	}
}

func TestIDTextAndJSONRoundTrips(t *testing.T) {
	locationID := LocationID{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0x4d, 0xef, 0x8e, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10}
	groupID := GroupID{0xff, 0xee, 0xdd, 0xcc, 0xbb, 0xaa, 0x49, 0x88, 0xb7, 0x66, 0x55, 0x44, 0x33, 0x22, 0x11, 0x00}

	locationText, err := locationID.MarshalText()
	require.NoError(t, err)
	var decodedLocation LocationID
	require.NoError(t, decodedLocation.UnmarshalText(locationText))
	assert.Equal(t, locationID, decodedLocation)

	encodedGroup, err := json.Marshal(groupID)
	require.NoError(t, err)
	assert.JSONEq(t, `"ffeeddcc-bbaa-4988-b766-554433221100"`, string(encodedGroup))
	var decodedGroup GroupID
	require.NoError(t, json.Unmarshal(encodedGroup, &decodedGroup))
	assert.Equal(t, groupID, decodedGroup)
}
