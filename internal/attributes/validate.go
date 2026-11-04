package attributes

import (
	"bytes"
	"encoding/json"
	"math"
	"slices"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/jnaranj09/open-customer/internal/platform/problem"
	"github.com/jnaranj09/open-customer/internal/platform/text"
)

const maxTextRunes = 500

// validateValues checks the attrs of one write against the definitions and merges them into the
// stored object. The touched keys are the keys of request; null removes a key on update and is
// ignored on create. A value equal to the stored one is kept without further checks, so a form
// that re-sends a value whose attribute or option was archived since still saves. The result has
// sorted keys. Errors are in key order, one per key.
func validateValues(defs []Attribute, scope Scope, stored, request json.RawMessage, isCreate bool) (json.RawMessage, []problem.FieldError) {
	storedObj, ok := decodeObject(stored)
	if !ok {
		return nil, attrsInvalid()
	}
	reqObj, ok := decodeObject(request)
	if !ok {
		return nil, attrsInvalid()
	}
	byKey := make(map[string]Attribute, len(defs))
	for _, d := range defs {
		byKey[d.Key] = d
	}
	merged := make(map[string]json.RawMessage, len(storedObj)+len(reqObj))
	for k, v := range storedObj {
		merged[k] = v
	}
	failed := map[string]string{}
	keys := make([]string, 0, len(reqObj))
	for k := range reqObj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		raw := compact(reqObj[key])
		def, known := byKey[key]
		switch {
		case !known:
			failed[key] = msg(problem.MsgAttrUnknown, key)
			continue
		case def.Scope != scope:
			failed[key] = msg(problem.MsgAttrOtherScope, def.Label)
			continue
		}
		isNull := string(raw) == "null"
		if isCreate && isNull {
			continue
		}
		if old, has := storedObj[key]; has && bytes.Equal(compact(old), raw) {
			continue
		}
		if def.Archived {
			failed[key] = msg(problem.MsgAttrArchived, def.Label)
			continue
		}
		if isNull {
			delete(merged, key)
			continue
		}
		value, remove, errMsg := checkValue(def, raw, storedObj[key])
		switch {
		case errMsg != "":
			failed[key] = errMsg
		case remove:
			delete(merged, key)
		default:
			merged[key] = value
		}
	}
	for _, d := range defs {
		if !d.Required || d.Archived || d.Scope != scope {
			continue
		}
		_, touched := reqObj[d.Key]
		if _, has := merged[d.Key]; has || failed[d.Key] != "" || (!isCreate && !touched) {
			continue
		}
		failed[d.Key] = msg(problem.MsgAttrRequired, d.Label)
	}
	if len(failed) > 0 {
		return nil, sortedErrors(failed)
	}
	out, err := json.Marshal(merged)
	if err != nil {
		return nil, attrsInvalid()
	}
	return out, nil
}

func attrsInvalid() []problem.FieldError {
	return []problem.FieldError{{Field: "attrs", Message: problem.MsgAttrsInvalid}}
}

func sortedErrors(failed map[string]string) []problem.FieldError {
	keys := make([]string, 0, len(failed))
	for k := range failed {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	errs := make([]problem.FieldError, 0, len(keys))
	for _, k := range keys {
		errs = append(errs, problem.FieldError{Field: "attrs." + k, Message: failed[k]})
	}
	return errs
}

// decodeObject reads empty input as {} and rejects anything but a JSON object.
func decodeObject(raw json.RawMessage) (map[string]json.RawMessage, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return map[string]json.RawMessage{}, true
	}
	if trimmed[0] != '{' {
		return nil, false
	}
	var obj map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.UseNumber()
	if err := dec.Decode(&obj); err != nil || obj == nil {
		return nil, false
	}
	return obj, true
}

func compact(raw json.RawMessage) json.RawMessage {
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return raw
	}
	return buf.Bytes()
}

// checkValue validates one non-null new value. It returns the value to store, or remove, or the
// error message.
func checkValue(def Attribute, raw, stored json.RawMessage) (value json.RawMessage, remove bool, errMsg string) {
	switch def.Type {
	case TypeText:
		return checkText(def, raw)
	case TypeNumber:
		return checkNumber(def, raw)
	case TypeDate:
		return checkDate(def, raw)
	case TypeSelect:
		return checkSelect(def, raw)
	case TypeMultiselect:
		return checkMultiselect(def, raw, stored)
	}
	return nil, false, problem.MsgAttrsInvalid
}

func asString(raw json.RawMessage) (string, bool) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", false
	}
	return s, true
}

func checkText(def Attribute, raw json.RawMessage) (json.RawMessage, bool, string) {
	s, ok := asString(raw)
	if !ok {
		return nil, false, msg(problem.MsgAttrTextInvalid, def.Label)
	}
	s, ok = text.Trim(s, 0, maxTextRunes)
	if !ok {
		return nil, false, msg(problem.MsgAttrTextInvalid, def.Label)
	}
	if s == "" {
		return nil, true, ""
	}
	out, err := json.Marshal(s)
	if err != nil {
		return nil, false, msg(problem.MsgAttrTextInvalid, def.Label)
	}
	return out, false, ""
}

func checkNumber(def Attribute, raw json.RawMessage) (json.RawMessage, bool, string) {
	if _, ok := parseNumber(raw); !ok {
		return nil, false, msg(problem.MsgAttrNumberInvalid, def.Label)
	}
	return raw, false, ""
}

// parseNumber reads a JSON number (not a string holding one) as a finite float.
func parseNumber(raw json.RawMessage) (float64, bool) {
	var v any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return 0, false
	}
	n, isNum := v.(json.Number)
	if !isNum {
		return 0, false
	}
	f, err := strconv.ParseFloat(n.String(), 64)
	if err != nil || math.IsInf(f, 0) {
		return 0, false
	}
	return f, true
}

func checkDate(def Attribute, raw json.RawMessage) (json.RawMessage, bool, string) {
	s, ok := asString(raw)
	if !ok {
		return nil, false, msg(problem.MsgAttrDateInvalid, def.Label)
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil || t.Format(time.DateOnly) != s {
		return nil, false, msg(problem.MsgAttrDateInvalid, def.Label)
	}
	return raw, false, ""
}

// optionOf finds the option of def named by a string ID, in canonical form.
func optionOf(def Attribute, id any) (Option, string, bool) {
	s, ok := id.(string)
	if !ok {
		return Option{}, "", false
	}
	u, err := uuid.Parse(s)
	if err != nil {
		return Option{}, "", false
	}
	for _, o := range def.Options {
		if o.ID == u {
			return o, u.String(), true
		}
	}
	return Option{}, "", false
}

func checkSelect(def Attribute, raw json.RawMessage) (json.RawMessage, bool, string) {
	s, ok := asString(raw)
	if !ok {
		return nil, false, msg(problem.MsgAttrOptionInvalid, def.Label)
	}
	opt, canon, ok := optionOf(def, s)
	if !ok {
		return nil, false, msg(problem.MsgAttrOptionInvalid, def.Label)
	}
	if opt.Archived {
		return nil, false, msg(problem.MsgAttrOptionArchived, def.Label)
	}
	out, err := json.Marshal(canon)
	if err != nil {
		return nil, false, msg(problem.MsgAttrOptionInvalid, def.Label)
	}
	return out, false, ""
}

func checkMultiselect(def Attribute, raw, stored json.RawMessage) (json.RawMessage, bool, string) {
	var ids []any
	if err := json.Unmarshal(raw, &ids); err != nil {
		return nil, false, msg(problem.MsgAttrOptionInvalid, def.Label)
	}
	if len(ids) == 0 {
		return nil, true, ""
	}
	var old []string
	if err := json.Unmarshal(stored, &old); err != nil {
		old = nil // a missing or malformed stored value means nothing is stored
	}
	seen := make([]string, 0, len(ids))
	for _, id := range ids {
		opt, canon, ok := optionOf(def, id)
		switch {
		case !ok:
			return nil, false, msg(problem.MsgAttrOptionInvalid, def.Label)
		case slices.Contains(seen, canon):
			return nil, false, msg(problem.MsgAttrOptionRepeated, def.Label)
		case opt.Archived && !slices.Contains(old, canon):
			return nil, false, msg(problem.MsgAttrOptionArchived, def.Label)
		}
		seen = append(seen, canon)
	}
	out, err := json.Marshal(seen)
	if err != nil {
		return nil, false, msg(problem.MsgAttrOptionInvalid, def.Label)
	}
	return out, false, ""
}
