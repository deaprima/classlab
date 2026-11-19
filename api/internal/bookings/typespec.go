package bookings

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"classlab/api/internal/httpx"
)

// typeSpec is what one room type asks of a booking beyond the policy table (brief BR-20): extra
// requirements and the shape of the free-form details object. Adding a field for a type means adding it
// here, not branching on the type elsewhere.
type typeSpec struct {
	// participantsRequired makes the head count mandatory (an auditorium needs one, BR-21).
	participantsRequired bool
	// detailFields lists the keys of the details object this type accepts. The MVP defines none, so only
	// an empty object is valid; the place is here so such fields can be added without a schema change.
	detailFields []string
}

var typeSpecs = map[string]typeSpec{
	"AUDITORIUM": {participantsRequired: true},
}

func specFor(roomType string) typeSpec { return typeSpecs[roomType] }

// normalizeDetails validates the details object of a booking and returns it as compact JSON. A missing
// value means "{}". It must be a JSON object, and every key must be one the room type defines.
func normalizeDetails(roomType string, raw json.RawMessage) ([]byte, []httpx.ErrorDetail) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return []byte("{}"), nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, []httpx.ErrorDetail{{Field: "details", Message: "details harus berupa objek JSON."}}
	}
	allowed := specFor(roomType).detailFields
	var unknown []string
	for k := range obj {
		if !slices.Contains(allowed, k) {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		slices.Sort(unknown)
		return nil, []httpx.ErrorDetail{{Field: "details", Message: fmt.Sprintf("Field %s belum didukung untuk tipe ruang ini.", strings.Join(unknown, ", "))}}
	}
	out, err := json.Marshal(obj)
	if err != nil {
		return nil, []httpx.ErrorDetail{{Field: "details", Message: "details tidak dapat dibaca."}}
	}
	return out, nil
}
