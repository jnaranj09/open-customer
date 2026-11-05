package httpapi

import (
	"reflect"
	"strings"
	"testing"

	"github.com/jnaranj09/open-customer/internal/exporter"
	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
)

// jsonKeys maps each json tag name of struct type t to whether the tag has omitempty. A field with
// no tag or a "-" tag is not a key.
func jsonKeys(t *testing.T, typ reflect.Type) map[string]bool {
	t.Helper()
	keys := map[string]bool{}
	for f := range typ.Fields() {
		tag, ok := f.Tag.Lookup("json")
		if !ok || tag == "-" {
			continue
		}
		name, opts, _ := strings.Cut(tag, ",")
		keys[name] = strings.Contains(","+opts+",", ",omitempty,")
	}
	return keys
}

// TestExportJSONMatchesSchemas keeps the datos.json element types equal to the API schemas: the
// exporter may not import gen, so this is the only link between the two.
func TestExportJSONMatchesSchemas(t *testing.T) {
	t.Parallel()
	pairs := []struct {
		name     string
		exporter any
		schema   any
	}{
		{"Settings", exporter.Settings{}, gen.Settings{}},
		{"Attribute", exporter.Attribute{}, gen.Attribute{}},
		{"Option", exporter.Option{}, gen.Option{}},
		{"Service", exporter.Service{}, gen.Service{}},
		{"Template", exporter.Template{}, gen.Template{}},
		{"Rule", exporter.Rule{}, gen.Rule{}},
		{"Client", exporter.Client{}, gen.Client{}},
		{"Appointment", exporter.Appointment{}, gen.Appointment{}},
		{"AppointmentService", exporter.AppointmentService{}, gen.AppointmentService{}},
		{"LogItem", exporter.LogItem{}, gen.LogItem{}},
	}
	for _, p := range pairs {
		t.Run(p.name, func(t *testing.T) {
			t.Parallel()
			got, want := jsonKeys(t, reflect.TypeOf(p.exporter)), jsonKeys(t, reflect.TypeOf(p.schema))
			if !reflect.DeepEqual(got, want) {
				t.Errorf("exporter.%s json keys (true = omitempty) = %v, gen.%s = %v", p.name, got, p.name, want)
			}
		})
	}
}
