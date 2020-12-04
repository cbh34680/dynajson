package dynajson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"runtime"
	"sort"
	"strings"
)

// Dump ...https://pod.hatenablog.com/entry/2016/05/15/232710
func Dump(d *interface{}, buf *bytes.Buffer) {
	switch v := (*d).(type) {
	// * add [pointer of array] -->
	case *[]interface{}:
		var i interface{} = *v
		//i = *v
		Dump(&i, buf)
		// * add [pointer of array] <--
	case []interface{}:
		buf.WriteString("[")
		for _, sub := range v {
			Dump(&sub, buf)
			buf.WriteString(", ")
		}
		if len(v) > 0 {
			buf.Truncate(buf.Len() - 2)
		}
		buf.WriteString("]")
	case map[string]interface{}:
		buf.WriteString("{")
		for k, sub := range v {
			buf.WriteString(fmt.Sprintf(`"%s"`, k))
			buf.WriteString(": ")
			Dump(&sub, buf)
			buf.WriteString(", ")
		}
		if len(v) > 0 {
			buf.Truncate(buf.Len() - 2)
		}
		buf.WriteString("}")
	case string:
		// * add escape -->
		bb := bytes.Buffer{}
		for _, r := range v {

			switch r {
			case 34, 92: // ["] [\]
				bb.WriteRune(92)
			}
			bb.WriteRune(r)
		}
		v = bb.String()
		// * add escape <--
		buf.WriteString(fmt.Sprintf(`"%s"`, v))
	default:
		buf.WriteString(fmt.Sprintf("%v", v))
	}
}

// JSONElement ... struct
type JSONElement struct {
	rawObject   interface{}
	WarnHandler func(*JSONElement, string, string, int)
	Level       int
	Readonly    bool
}

// ---------------------------------------------------------------------------

// New ... func
func New(obj interface{}) *JSONElement {

	return &JSONElement{
		rawObject: obj,
	}
}

// NewAsMap ... func
func NewAsMap() *JSONElement {
	return New(map[string]interface{}{})
}

// NewRootAsMap ... rename
func NewRootAsMap() *JSONElement {
	return NewAsMap()
}

// NewAsArray ... func
func NewAsArray() *JSONElement {
	return New(&[]interface{}{})
}

// NewRootAsArray ... rename
func NewRootAsArray() *JSONElement {
	return NewAsArray()
}

// NewByBytes ... func
func NewByBytes(data []byte) (*JSONElement, error) {

	var obj interface{}

	err := json.Unmarshal(data, &obj)
	if err != nil {
		return nil, fmt.Errorf("Unmarshal: %w", err)
	}

	return New(obj), nil
}

// NewByString ... func
func NewByString(data string) (*JSONElement, error) {

	return NewByBytes([]byte(data))
}

// NewByPath ... func
func NewByPath(path string) (*JSONElement, error) {

	var data []byte

	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {

		// https://golang.hateblo.jp/entry/golang-http-request
		// https://qiita.com/ono_matope/items/60e96c01b43c64ed1d18
		// https://qiita.com/stk0724/items/dc400dccd29a4b3d6471

		req, err := http.NewRequest(http.MethodGet, path, nil)
		if err != nil {
			return nil, fmt.Errorf("http.NewRequest: %s: %w", path, err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http.DefaultClient.Do: %s: %w", path, err)
		}
		defer func() {
			io.Copy(ioutil.Discard, resp.Body)
			resp.Body.Close()
		}()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("StatusCode != 200: %s: %d", path, resp.StatusCode)
		}

		bytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("ReadAll: %s: %w", path, err)
		}

		data = bytes
	} else {

		bytes, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("ReadFile: %s: %w", path, err)
		}

		data = bytes
	}

	return NewByBytes(data)
}

// ---------------------------------------------------------------------------

// Warn ... func
func (me *JSONElement) Warn(format string, a ...interface{}) {

	if me.WarnHandler == nil {
		return
	}

	_, where, line, _ := runtime.Caller(2)

	me.WarnHandler(me, fmt.Sprintf(format, a...), where, line)
}

// Errorf ... func
func (me *JSONElement) Errorf(format string, a ...interface{}) error {

	err := fmt.Errorf(format, a...)
	me.Warn(err.Error())

	return err
}

// Raw ... func
func (me *JSONElement) Raw() interface{} {

	if me == nil {
		return nil
	}

	return me.rawObject
}

// ---------------------------------------------------------------------------

// Put ... func
func (me *JSONElement) Put(key string, val1 interface{}, vals ...interface{}) error {

	if me.rawObject == nil {
		return me.Errorf("key=[%s]: me.rawObject is null", key)
	}

	if me.Readonly {
		return me.Errorf("key=[%s]: me.Readonly is true", key)
	}

	typedObj, ok := me.rawObject.(map[string]interface{})
	if !ok {
		return me.Errorf("key=[%s]: Not Map Type: %T", key, me.rawObject)
	}

	switch len(vals) {
	case 0:
		// 一つの時は "key": val
		typedObj[key] = val1
	default:
		// 複数の時は "key": [val, val, ...]
		arr := []interface{}{val1}
		arr = append(arr, vals...)
		typedObj[key] = &arr
	}

	return nil
}

// PutEmptyMap ... func
func (me *JSONElement) PutEmptyMap(key string) (*JSONElement, error) {

	if me.rawObject == nil {
		return nil, me.Errorf("key=[%s]: me.rawObject is null", key)
	}

	if me.Readonly {
		return nil, me.Errorf("key=[%s]: me.Readonly is true", key)
	}

	err := me.Put(key, map[string]interface{}{})
	if err != nil {
		return nil, me.Errorf("key=[%s]: %w", key, err)
	}

	return me.SelectByKey(key), nil
}

// PutEmptyArray ... func
func (me *JSONElement) PutEmptyArray(key string) (*JSONElement, error) {

	if me.rawObject == nil {
		return nil, me.Errorf("key=[%s]: me.rawObject is null", key)
	}

	if me.Readonly {
		return nil, me.Errorf("key=[%s]: me.Readonly is true", key)
	}

	err := me.Put(key, &[]interface{}{})
	if err != nil {
		return nil, me.Errorf("key=[%s]: Put: %w", key, err)
	}

	return me.SelectByKey(key), nil
}

// Append ... func
func (me *JSONElement) Append(val1 interface{}, vals ...interface{}) error {

	if me.rawObject == nil {
		return me.Errorf("me.rawObject is null")
	}

	if me.Readonly {
		return me.Errorf("me.Readonly is true")
	}

	typedObj, ok := me.rawObject.(*[]interface{})
	if !ok {
		return me.Errorf("Not Editable-Array Type: %T", me.rawObject)
	}

	(*typedObj) = append((*typedObj), val1)

	if len(vals) != 0 {
		(*typedObj) = append((*typedObj), vals...)
	}

	return nil
}

// DeleteByKey ... func
func (me *JSONElement) DeleteByKey(key string) error {

	if me.rawObject == nil {
		return me.Errorf("key=[%s]: me.rawObject is null", key)
	}

	if me.Readonly {
		return me.Errorf("key=[%s]: me.Readonly is true", key)
	}

	typedObj, ok := me.rawObject.(map[string]interface{})
	if !ok {
		return me.Errorf("key=[%s]: Not Map Type: %T", key, me.rawObject)
	}

	if _, ok := typedObj[key]; !ok {
		me.Warn("DeleteByKey(%s): No Key", key)
		return nil
	}

	delete(typedObj, key)

	return nil
}

// https://www.delftstack.com/ja/howto/go/how-to-delete-an-element-from-a-slice-in-golang/
func remove(slice []interface{}, s int) []interface{} {
	return append(slice[:s], slice[s+1:]...)
}

// DeleteByPos ... func
func (me *JSONElement) DeleteByPos(pos int) error {

	if me.rawObject == nil {
		return me.Errorf("me.rawObject is null")
	}

	if me.Readonly {
		return me.Errorf("pos=[%d]: me.Readonly is true", pos)
	}

	typedObj, ok := me.rawObject.(*[]interface{})
	if !ok {
		return me.Errorf("pos=[%d]: Not Editable-Array Type: %T", pos, me.rawObject)
	}

	containerLen := len(*typedObj)

	if pos >= containerLen {
		me.Warn("DeleteByPos(%d): Overflow: Container(%d)", pos, containerLen)
		return nil
	}

	(*typedObj) = remove(*typedObj, pos)

	return nil
}

// Delete ... func
func (me *JSONElement) Delete(arg interface{}) error {

	if me.rawObject == nil {
		return me.Errorf("me.rawObject is null")
	}

	switch v := arg.(type) {
	case int:
		return me.DeleteByPos(v)
	case string:
		return me.DeleteByKey(v)
	}

	return me.Errorf("Bad Argument Type: %T", arg)
}

// ---------------------------------------------------------------------------

func (me *JSONElement) child(obj interface{}) *JSONElement {

	return &JSONElement{
		rawObject:   obj,
		WarnHandler: me.WarnHandler,
		Level:       me.Level + 1,
		Readonly:    me.Readonly,
	}
}

func (me *JSONElement) String() string {

	buf := &bytes.Buffer{}

	if me.rawObject != nil {
		Dump(&me.rawObject, buf)
	}

	return buf.String()
}

// Count ... func
func (me *JSONElement) Count() int {

	if me.rawObject == nil {
		me.Warn("Count: Null Object")
		return 0
	}

	switch v := me.rawObject.(type) {
	case map[string]interface{}:
		return len(v)
	case []interface{}:
		return len(v)
	case *[]interface{}:
		return len(*v)
	}

	me.Warn("Count: Not Container: %T", me.rawObject)
	return 1
}

// SelectByKey ... func
func (me *JSONElement) SelectByKey(key string) *JSONElement {

	if me.rawObject == nil {
		me.Warn("SelectByKey(%s): Null Object", key)
		return me.child(nil)
	}

	typedObj, ok := me.rawObject.(map[string]interface{})
	if !ok {
		me.Warn("SelectByKey(%s): Cast: %T", key, me.rawObject)
		return me.child(nil)
	}

	return me.child(typedObj[key])
}

// SelectByPos ... func
func (me *JSONElement) SelectByPos(pos int) *JSONElement {

	if me.rawObject == nil {
		me.Warn("SelectByPos(%d): Null Object", pos)
		return me.child(nil)
	}

	var typedObj []interface{}

	switch v := me.rawObject.(type) {
	case []interface{}:
		typedObj = v
	case *[]interface{}:
		typedObj = *v
	default:
		me.Warn("SelectByPos(%d): Not Array: %T", pos, me.rawObject)
		return me.child(nil)
	}

	containerLen := len(typedObj)

	if pos >= containerLen {
		me.Warn("SelectByPos(%d): Overflow: %d", pos, containerLen)

		return me.child(nil)
	}

	return me.child(typedObj[pos])
}

// Select ... func
func (me *JSONElement) Select(arg interface{}) *JSONElement {

	if me.rawObject == nil {
		me.Warn("Select(%v): Null Object", arg)
		return me.child(nil)
	}

	switch v := arg.(type) {
	case int:
		return me.SelectByPos(v)
	case string:
		return me.SelectByKey(v)
	}

	me.Warn("Select(%v): Cast: %[1]T", arg)
	return me.child(nil)
}

// ---------------------------------------------------------------------------

// Keys ... func
func (me *JSONElement) Keys() []string {

	if me.rawObject == nil {
		me.Warn("Keys: Null Object")
		return []string{}
	}

	typedObj, ok := me.rawObject.(map[string]interface{})
	if !ok {
		me.Warn("Keys: Cast: %T", me.rawObject)
		return []string{}
	}

	keys := make([]string, len(typedObj))

	i := 0
	for k := range typedObj {
		keys[i] = k
		i++
	}

	return keys
}

// EachMap ... func
func (me *JSONElement) EachMap(callback func(string, *JSONElement)) {

	if me.rawObject == nil {
		me.Warn("EachMap: Null Object")
		return
	}

	typedObj, ok := me.rawObject.(map[string]interface{})
	if !ok {
		me.Warn("EachMap: Cast: %T", me.rawObject)
		return
	}

	containerLen := len(typedObj)
	if containerLen == 0 {
		return
	}

	keys := make([]string, containerLen)

	i := 0
	for k := range typedObj {

		keys[i] = k
		i++
	}

	sort.Strings(keys)

	for _, k := range keys {
		callback(k, me.child(typedObj[k]))
	}
}

// EachArray ... func
func (me *JSONElement) EachArray(callback func(int, *JSONElement)) {

	if me.rawObject == nil {
		me.Warn("EachArray: Null Object")
		return
	}

	var typedObj []interface{}
	switch v := me.rawObject.(type) {
	case []interface{}:
		typedObj = v
	case *[]interface{}:
		typedObj = *v
	default:
		me.Warn("EachArray: Cast: %T", me.rawObject)
		return
	}

	for i, v := range typedObj {
		callback(i, me.child(v))
	}
}

// ---------------------------------------------------------------------------

// AsArray ... func
func (me *JSONElement) AsArray() []*JSONElement {

	if me.rawObject == nil {
		me.Warn("AsArray: Null Object")
		return []*JSONElement{}
	}

	var typedObj []interface{}
	switch v := me.rawObject.(type) {
	case []interface{}:
		typedObj = v
	case *[]interface{}:
		typedObj = *v
	default:
		me.Warn("AsArray: Cast: %T", me.rawObject)
		return []*JSONElement{}
	}

	arr := make([]*JSONElement, len(typedObj))
	for i, v := range typedObj {
		arr[i] = me.child(v)
	}

	return arr
}

// AsString ... func
func (me *JSONElement) AsString() string {

	if me.rawObject == nil {
		me.Warn("AsString: Null Object")
		return ""
	}

	typedObj, ok := me.rawObject.(string)
	if !ok {
		me.Warn("AsString: Cast: %T", me.rawObject)
		return ""
	}

	return typedObj
}

// AsBool ... func
func (me *JSONElement) AsBool() bool {

	if me.rawObject == nil {
		me.Warn("AsBool: Null Object")
		return false
	}

	typedObj, ok := me.rawObject.(bool)
	if !ok {
		me.Warn("AsBool: Cast: %T", me.rawObject)
		return false
	}

	return typedObj
}

// AsInt ... func
func (me *JSONElement) AsInt() int {

	if me.rawObject == nil {
		me.Warn("AsInt: Null Object")
		return 0
	}

	var rv int

	switch v := me.rawObject.(type) {
	case int:
		rv = v
	case float64:
		rv = int(v)
	default:
		me.Warn("AsInt: Cast: %T", me.rawObject)
	}

	return rv
}

// AsFloat ... func
func (me *JSONElement) AsFloat() float64 {

	if me.rawObject == nil {
		me.Warn("AsFloat: Null Object")
		return 0.0
	}

	var rv float64

	switch v := me.rawObject.(type) {
	case int:
		rv = float64(v)
	case float64:
		rv = v
	default:
		me.Warn("AsInt: Cast: %T", me.rawObject)
	}

	return rv
}
