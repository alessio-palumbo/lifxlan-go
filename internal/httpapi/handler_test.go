package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/alessio-palumbo/lifxlan-go/internal/control"
	"github.com/alessio-palumbo/lifxlan-go/pkg/controller"
	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

func TestGetDevicesIncludesCapabilityScopedState(t *testing.T) {
	light := apiDevice(t, "001122334455", device.DeviceTypeLight)
	light.LocationID = device.LocationID{1}
	light.GroupID = device.GroupID{2}
	light.PoweredOn = true
	light.Color = device.Color{Hue: 210, Saturation: 80, Brightness: 60, Kelvin: 3500}
	switchDevice := apiDevice(t, "aabbccddeeff", device.DeviceTypeSwitch)
	switchDevice.Relays = []device.Relay{{Index: 0, PoweredOn: true}}
	handler := NewHandler(&fakeService{devices: []device.Device{light, switchDevice}})

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/devices", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body struct {
		Devices []deviceResponse `json:"devices"`
	}
	decodeResponse(t, response, &body)
	if len(body.Devices) != 2 {
		t.Fatalf("devices = %d, want 2", len(body.Devices))
	}
	if body.Devices[0].LocationID != light.LocationID.String() || body.Devices[0].GroupID != light.GroupID.String() {
		t.Fatalf("light identifiers = %+v", body.Devices[0])
	}
	if body.Devices[0].Light == nil || body.Devices[0].Switch != nil || body.Devices[0].Light.Power != "on" {
		t.Fatalf("light capabilities = %+v", body.Devices[0])
	}
	if body.Devices[1].Light != nil || body.Devices[1].Switch == nil || len(body.Devices[1].Switch.Relays) != 1 {
		t.Fatalf("switch capabilities = %+v", body.Devices[1])
	}
}

func TestGetDeviceValidatesSerialAndReturnsNotFound(t *testing.T) {
	device_ := apiDevice(t, "001122334455", device.DeviceTypeLight)
	handler := NewHandler(&fakeService{devices: []device.Device{device_}})

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/devices/invalid", nil))
	assertErrorCode(t, response, http.StatusBadRequest, "invalid_serial")

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/devices/aabbccddeeff", nil))
	assertErrorCode(t, response, http.StatusNotFound, "device_not_found")

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/devices/"+device_.Serial.String(), nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestGetDeviceEventsStreamsSSE(t *testing.T) {
	light := apiDevice(t, "001122334455", device.DeviceTypeLight)
	light.PoweredOn = true
	events := make(chan controller.DeviceEvent, 2)
	events <- controller.DeviceEvent{
		Type:     controller.DeviceEventUpdated,
		Device:   light,
		Changes:  controller.DeviceChangeLight,
		Revision: 7,
	}
	events <- controller.DeviceEvent{
		Type:     controller.DeviceEventResyncRequired,
		Revision: 8,
	}
	close(events)

	handler := NewHandler(&fakeService{events: events})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/devices/events", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("content type = %q", got)
	}
	body := response.Body.String()
	for _, want := range []string{
		"id: 7\nevent: updated\n",
		"\"type\":\"updated\"",
		"\"changes\":[\"light\"]",
		"\"serial\":\"001122334455\"",
		"id: 8\nevent: resync_required\n",
		"data: {\"type\":\"resync_required\",\"revision\":8}\n\n",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("SSE body does not contain %q:\n%s", want, body)
		}
	}
}

func TestPatchDeviceStateMapsRequestAndResults(t *testing.T) {
	serial := mustAPISerial(t, "001122334455")
	service := &fakeService{results: []control.DeviceStateResult{
		{Serial: serial, Status: control.ApplyAccepted},
		{Serial: mustAPISerial(t, "aabbccddeeff"), Status: control.ApplyPartial, Sent: 1, Total: 2, Err: errors.New("send failed")},
	}}
	handler := NewHandler(service)
	body := []byte(`{
		"selectors":["group_id:00000000-0000-4000-8000-000000000001"],
		"light":{"power":"on","color":{"hue":210,"brightness":60,"kelvin":3500},"duration_ms":500},
		"relays":[{"index":0,"power":"off"}]
	}`)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPatch, "/v1/devices/state", bytes.NewReader(body)))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if !reflect.DeepEqual(service.selectors, []string{"group_id:00000000-0000-4000-8000-000000000001"}) {
		t.Fatalf("selectors = %v", service.selectors)
	}
	if service.update.Light == nil || service.update.Light.Power == nil || !*service.update.Light.Power || service.update.Light.Hue == nil || service.update.Light.Duration.Milliseconds() != 500 {
		t.Fatalf("light update = %+v", service.update.Light)
	}
	if len(service.update.Relays) != 1 || service.update.Relays[0].PoweredOn {
		t.Fatalf("relay update = %+v", service.update.Relays)
	}
	var responseBody struct {
		Results []stateResultResponse `json:"results"`
	}
	decodeResponse(t, response, &responseBody)
	if responseBody.Results[1].Status != control.ApplyPartial || responseBody.Results[1].Sent != 1 || responseBody.Results[1].Error == "" {
		t.Fatalf("partial result = %+v", responseBody.Results[1])
	}
}

func TestPatchDeviceStateRejectsMalformedRequests(t *testing.T) {
	handler := NewHandler(&fakeService{})
	tests := []struct {
		name string
		body string
		code string
	}{
		{name: "unknown field", body: `{"selectors":["all"],"unknown":true}`, code: "invalid_request"},
		{name: "no state", body: `{"selectors":["all"]}`, code: "invalid_state"},
		{name: "empty light", body: `{"selectors":["all"],"light":{}}`, code: "invalid_state"},
		{name: "invalid power", body: `{"selectors":["all"],"light":{"power":"yes"}}`, code: "invalid_state"},
		{name: "invalid duration", body: `{"selectors":["all"],"light":{"power":"on","duration_ms":-1}}`, code: "invalid_state"},
		{name: "brightness out of range", body: `{"selectors":["all"],"light":{"color":{"brightness":101}}}`, code: "invalid_state"},
		{name: "duplicate relay", body: `{"selectors":["all"],"relays":[{"index":0,"power":"on"},{"index":0,"power":"off"}]}`, code: "invalid_state"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodPatch, "/v1/devices/state", bytes.NewBufferString(tt.body)))
			assertErrorCode(t, response, http.StatusBadRequest, tt.code)
		})
	}
}

func TestPatchDeviceStateMapsSelectorErrors(t *testing.T) {
	tests := []struct {
		err    error
		status int
		code   string
	}{
		{err: control.ErrInvalidSelectors, status: http.StatusBadRequest, code: "invalid_selectors"},
		{err: control.ErrNoDevicesMatched, status: http.StatusNotFound, code: "no_devices_matched"},
	}
	for _, tt := range tests {
		service := &fakeService{applyErr: tt.err}
		handler := NewHandler(service)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodPatch, "/v1/devices/state", bytes.NewBufferString(`{"selectors":["all"],"light":{"power":"on"}}`)))
		assertErrorCode(t, response, tt.status, tt.code)
	}
}

type fakeService struct {
	devices   []device.Device
	events    <-chan controller.DeviceEvent
	selectors []string
	update    controller.StateUpdate
	results   []control.DeviceStateResult
	applyErr  error
}

func (s *fakeService) GetDevices() []device.Device {
	return append([]device.Device(nil), s.devices...)
}

func (s *fakeService) GetDevice(serial device.Serial) (device.Device, bool) {
	for _, d := range s.devices {
		if d.Serial == serial {
			return d, true
		}
	}
	return device.Device{}, false
}

func (s *fakeService) ApplyState(selectors []string, update controller.StateUpdate) ([]control.DeviceStateResult, error) {
	s.selectors = append([]string(nil), selectors...)
	s.update = update
	return s.results, s.applyErr
}

func (s *fakeService) SubscribeDevices(context.Context, ...controller.SubscriptionOption) <-chan controller.DeviceEvent {
	if s.events != nil {
		return s.events
	}
	events := make(chan controller.DeviceEvent)
	close(events)
	return events
}

func apiDevice(t *testing.T, serial string, deviceType device.DeviceType) device.Device {
	t.Helper()
	return device.Device{
		Serial:        mustAPISerial(t, serial),
		Label:         "Device",
		Type:          deviceType,
		LightType:     device.LightTypeSingleZone,
		Location:      "Home",
		Group:         "Office",
		RegistryKnown: true,
		ColorProperties: device.ColorProperties{
			HasColor:         true,
			TemperatureRange: device.TemperatureRange{Min: 1500, Max: 9000},
		},
	}
}

func mustAPISerial(t *testing.T, value string) device.Serial {
	t.Helper()
	serial, err := device.SerialFromHex(value)
	if err != nil {
		t.Fatal(err)
	}
	return serial
}

func decodeResponse(t *testing.T, response *httptest.ResponseRecorder, dst any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), dst); err != nil {
		t.Fatalf("decode response: %v; body = %s", err, response.Body.String())
	}
}

func assertErrorCode(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, status, response.Body.String())
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeResponse(t, response, &body)
	if body.Error.Code != code {
		t.Fatalf("error code = %q, want %q", body.Error.Code, code)
	}
}
