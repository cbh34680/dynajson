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
	raw          interface{}
	WarnHandler  func(*JSONElement, string, string, int)
	FatalHandler func(*JSONElement, string, string, int)
	Level        int
	Readonly     bool
}

// ---------------------------------------------------------------------------

// New ... func
func New(obj interface{}) *JSONElement {

	return &JSONElement{
		raw: obj,
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
func NewByPath(argPath string) (*JSONElement, error) {

	var data []byte

	if strings.HasPrefix(argPath, "http://") || strings.HasPrefix(argPath, "https://") {

		// https://golang.hateblo.jp/entry/golang-http-request
		// https://qiita.com/ono_matope/items/60e96c01b43c64ed1d18
		// https://qiita.com/stk0724/items/dc400dccd29a4b3d6471

		req, err := http.NewRequest(http.MethodGet, argPath, nil)
		if err != nil {
			return nil, fmt.Errorf("http.NewRequest: %s: %w", argPath, err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http.DefaultClient.Do: %s: %w", argPath, err)
		}
		defer func() {
			io.Copy(ioutil.Discard, resp.Body)
			resp.Body.Close()
		}()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("StatusCode != 200: %s: %d", argPath, resp.StatusCode)
		}

		bytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("ReadAll: %s: %w", argPath, err)
		}

		data = bytes
	} else {

		bytes, err := ioutil.ReadFile(argPath)
		if err != nil {
			return nil, fmt.Errorf("ReadFile: %s: %w", argPath, err)
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

// Fatal ... func
func (me *JSONElement) Fatal(format string, a ...interface{}) {

	if me.FatalHandler == nil {
		me.Warn(format, a...)
		return
	}

	_, where, line, _ := runtime.Caller(2)

	me.FatalHandler(me, fmt.Sprintf(format, a...), where, line)
}

// Errorf ... func
func (me *JSONElement) Errorf(format string, a ...interface{}) error {

	err := fmt.Errorf(format, a...)
	me.Fatal(err.Error())

	return err
}

// Raw ... func
func (me *JSONElement) Raw() interface{} {

	if me == nil {
		return nil
	}

	return me.raw
}

// IsNil ... func
func (me *JSONElement) IsNil() bool {

	return me.Raw() == nil
}

// ---------------------------------------------------------------------------

// Put ... func
func (me *JSONElement) Put(key string, val1 interface{}, vals ...interface{}) error {

	if me.IsNil() {
		return me.Errorf("key=[%s]: me.raw is null", key)
	}

	if me.Readonly {
		return me.Errorf("key=[%s]: me.Readonly is true", key)
	}

	typedObj, ok := me.raw.(map[string]interface{})
	if !ok {
		return me.Errorf("key=[%s]: Not Map Type: %T", key, me.raw)
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

	if me.IsNil() {
		return nil, me.Errorf("key=[%s]: me.raw is null", key)
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

	if me.IsNil() {
		return nil, me.Errorf("key=[%s]: me.raw is null", key)
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

	if me.IsNil() {
		return me.Errorf("me.raw is null")
	}

	if me.Readonly {
		return me.Errorf("me.Readonly is true")
	}

	typedObj, ok := me.raw.(*[]interface{})
	if !ok {
		return me.Errorf("Not Editable-Array Type: %T", me.raw)
	}

	(*typedObj) = append((*typedObj), val1)

	if len(vals) != 0 {
		(*typedObj) = append((*typedObj), vals...)
	}

	return nil
}

// DeleteByKey ... func
func (me *JSONElement) DeleteByKey(key string) error {

	if me.IsNil() {
		return me.Errorf("key=[%s]: me.raw is null", key)
	}

	if me.Readonly {
		return me.Errorf("key=[%s]: me.Readonly is true", key)
	}

	typedObj, ok := me.raw.(map[string]interface{})
	if !ok {
		return me.Errorf("key=[%s]: Not Map Type: %T", key, me.raw)
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

	if me.IsNil() {
		return me.Errorf("pos=[%d]: me.raw is null", pos)
	}

	if me.Readonly {
		return me.Errorf("pos=[%d]: me.Readonly is true", pos)
	}

	typedObj, ok := me.raw.(*[]interface{})
	if !ok {
		return me.Errorf("pos=[%d]: Not Editable-Array Type: %T", pos, me.raw)
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

	if me.IsNil() {
		return me.Errorf("me.raw is null")
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
		raw:         obj,
		WarnHandler: me.WarnHandler,
		Level:       me.Level + 1,
		Readonly:    me.Readonly,
	}
}

func (me *JSONElement) String() string {

	buf := &bytes.Buffer{}

	if me.raw != nil {
		Dump(&me.raw, buf)
	}

	return buf.String()
}

// Count ... func
func (me *JSONElement) Count() int {

	if me.IsNil() {
		me.Warn("Count: Null Object")
		return 0
	}

	switch v := me.raw.(type) {
	case map[string]interface{}:
		return len(v)
	case []interface{}:
		return len(v)
	case *[]interface{}:
		return len(*v)
	}

	me.Warn("Count: Not Container: %T", me.raw)
	return 1
}

// SelectByKey ... func
func (me *JSONElement) SelectByKey(key string) *JSONElement {

	if me.IsNil() {
		me.Warn("key=[%s]: SelectByKey: Null Object", key)
		return me.child(nil)
	}

	typedObj, ok := me.raw.(map[string]interface{})
	if !ok {
		me.Warn("key=[%s]: SelectByKey: Cast: %T", key, me.raw)
		return me.child(nil)
	}

	return me.child(typedObj[key])
}

// SelectByPos ... func
func (me *JSONElement) SelectByPos(pos int) *JSONElement {

	if me.IsNil() {
		me.Warn("pos=[%d]: SelectByPos: Null Object", pos)
		return me.child(nil)
	}

	var typedObj []interface{}

	switch v := me.raw.(type) {
	case []interface{}:
		typedObj = v
	case *[]interface{}:
		typedObj = *v
	default:
		me.Warn("pos=[%d]: SelectByPos: Not Array: %T", pos, me.raw)
		return me.child(nil)
	}

	containerLen := len(typedObj)

	if pos >= containerLen {
		me.Warn("pos=[%d]: SelectByPos: Overflow: %d", pos, containerLen)

		return me.child(nil)
	}

	return me.child(typedObj[pos])
}

// Select ... func
func (me *JSONElement) Select(key1 interface{}, keys ...interface{}) *JSONElement {

	if me.IsNil() {
		me.Warn("Select: Null Object")
		return me.child(nil)
	}

	if strArr, ok := key1.([]string); ok {

		newArgsLen := len(strArr) + len(keys)
		if newArgsLen == 0 {

			me.Warn("Select: No key")
			return me.child(nil)
		}

		newArgs := make([]interface{}, newArgsLen)

		i := 0
		for _, z := range strArr {
			newArgs[i] = z
			i++
		}
		for _, z := range keys {
			newArgs[i] = z
			i++
		}

		key1 = newArgs[0]
		keys = newArgs[1:]
	}

	var next *JSONElement

	switch x := key1.(type) {
	case int:
		next = me.SelectByPos(x)
	case string:
		next = me.SelectByKey(x)
	default:
		me.Warn("Select(%v): Cast: %[1]T", key1)
		return me.child(nil)
	}

	if len(keys) == 0 {
		return next
	}

	return next.Select(keys[0], keys[1:]...)
}

// ---------------------------------------------------------------------------

// Keys ... func
func (me *JSONElement) Keys() []string {

	if me.IsNil() {
		me.Warn("Keys: Null Object")
		return []string{}
	}

	typedObj, ok := me.raw.(map[string]interface{})
	if !ok {
		me.Warn("Keys: Cast: %T", me.raw)
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

	if me.IsNil() {
		me.Warn("EachMap: Null Object")
		return
	}

	typedObj, ok := me.raw.(map[string]interface{})
	if !ok {
		me.Warn("EachMap: Cast: %T", me.raw)
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

	if me.IsNil() {
		me.Warn("EachArray: Null Object")
		return
	}

	var typedObj []interface{}
	switch v := me.raw.(type) {
	case []interface{}:
		typedObj = v
	case *[]interface{}:
		typedObj = *v
	default:
		me.Warn("EachArray: Cast: %T", me.raw)
		return
	}

	for i, v := range typedObj {
		callback(i, me.child(v))
	}
}

type walkCallbackType func([]interface{}, interface{}, interface{})

// Walk ... func
func (me *JSONElement) Walk(callback walkCallbackType) {

	walk([]interface{}{}, me.raw, callback)
}

func walk(argParents []interface{}, argVal interface{}, callback walkCallbackType) {

	var arrObj []interface{}
	var mapObj map[string]interface{}

	switch v := argVal.(type) {
	case []interface{}:
		arrObj = v
	case *[]interface{}:
		arrObj = *v
	case map[string]interface{}:
		mapObj = v
	}

	if arrObj != nil {

		for k, v := range arrObj {
			callback(argParents, k, v)
			walk(append(argParents, k), v, callback)
		}
	}

	if mapObj != nil {
		for k, v := range mapObj {
			callback(argParents, k, v)
			walk(append(argParents, k), v, callback)
		}
	}
}

// ---------------------------------------------------------------------------

// AsArray ... func
func (me *JSONElement) AsArray() []*JSONElement {

	if me.IsNil() {
		me.Warn("AsArray: Null Object")
		return []*JSONElement{}
	}

	var typedObj []interface{}

	switch v := me.raw.(type) {
	case []interface{}:
		typedObj = v
	case *[]interface{}:
		typedObj = *v
	default:
		me.Warn("AsArray: Cast: %T", me.raw)
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

	if me.IsNil() {
		me.Warn("AsString: Null Object")
		return ""
	}

	typedObj, ok := me.raw.(string)
	if !ok {
		me.Warn("AsString: Cast: %T", me.raw)
		return ""
	}

	return typedObj
}

// AsBool ... func
func (me *JSONElement) AsBool() bool {

	if me.IsNil() {
		me.Warn("AsBool: Null Object")
		return false
	}

	typedObj, ok := me.raw.(bool)
	if !ok {
		me.Warn("AsBool: Cast: %T", me.raw)
		return false
	}

	return typedObj
}

// AsInt ... func
func (me *JSONElement) AsInt() int {

	if me.IsNil() {
		me.Warn("AsInt: Null Object")
		return 0
	}

	var rv int

	switch v := me.raw.(type) {
	case int:
		rv = v
	case float64:
		rv = int(v)
	default:
		me.Warn("AsInt: Cast: %T", me.raw)
	}

	return rv
}

// AsFloat ... func
func (me *JSONElement) AsFloat() float64 {

	if me.IsNil() {
		me.Warn("AsFloat: Null Object")
		return 0.0
	}

	var rv float64

	switch v := me.raw.(type) {
	case int:
		rv = float64(v)
	case float64:
		rv = v
	default:
		me.Warn("AsInt: Cast: %T", me.raw)
	}

	return rv
}
