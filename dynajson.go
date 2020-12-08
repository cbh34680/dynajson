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

func escapeJSONString(arg string) string {

	bb := bytes.Buffer{}
	for _, r := range arg {

		switch r {
		case 34, 92: // ["] [\]
			bb.WriteRune(92)
		}
		bb.WriteRune(r)
	}

	return bb.String()
}

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
			// * add escape -->
			//buf.WriteString(fmt.Sprintf(`"%s"`, k))
			buf.WriteString(fmt.Sprintf(`"%s"`, escapeJSONString(k)))
			// * add escape <--
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
		//buf.WriteString(fmt.Sprintf(`"%s"`, v))
		buf.WriteString(fmt.Sprintf(`"%s"`, escapeJSONString(v)))
		// * add escape <--
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

	_, where, line, _ := runtime.Caller(3)

	if me.FatalHandler == nil {

		if me.WarnHandler == nil {
			return
		}

		me.WarnHandler(me, fmt.Sprintf(format, a...), where, line)
		return
	}

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

// IsMap ... func
func (me *JSONElement) IsMap() bool {
	_, ok := me.Raw().(map[string]interface{})
	return ok
}

// IsArray ... func
func (me *JSONElement) IsArray() bool {
	switch me.Raw().(type) {
	case []interface{}, *[]interface{}:
		return true
	}
	return false
}

// ---------------------------------------------------------------------------

func elm2Raw(arg interface{}) interface{} {

	if elm, ok := arg.(*JSONElement); ok {
		return elm2Raw(elm.Raw())
	}

	return arg
}

func updateElms2Raws(arg []interface{}) {

	for i, v := range arg {

		arg[i] = elm2Raw(v)
	}
}

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
		typedObj[key] = elm2Raw(val1)
	default:
		// 複数の時は "key": [val, val, ...]
		arr := []interface{}{val1}
		arr = append(arr, vals...)
		updateElms2Raws(arr)

		typedObj[key] = &arr
	}

	return nil
}

// Append ... func
func (me *JSONElement) Append(val1 interface{}, vals ...interface{}) error {

	if me.IsNil() {
		return me.Errorf("me.raw is null")
	}

	if me.Readonly {
		return me.Errorf("me.Readonly is true")
	}

	refArr, ok := me.raw.(*[]interface{})
	if !ok {
		return me.Errorf("Not Editable-Array Type: %T", me.raw)
	}

	(*refArr) = append((*refArr), val1)

	if len(vals) != 0 {
		(*refArr) = append((*refArr), vals...)
	}

	updateElms2Raws(*refArr)

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

	refArr, ok := me.raw.(*[]interface{})
	if !ok {
		return me.Errorf("pos=[%d]: Not Editable-Array Type: %T", pos, me.raw)
	}

	containerLen := len(*refArr)

	if pos >= containerLen {
		me.Warn("DeleteByPos(%d): Overflow: Container(%d)", pos, containerLen)
		return nil
	}

	(*refArr) = remove(*refArr, pos)

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

func (me *JSONElement) child(raw interface{}) *JSONElement {

	return &JSONElement{
		raw:         raw,
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

	var refArr *[]interface{}

	switch v := me.raw.(type) {
	case []interface{}:
		refArr = &v
	case *[]interface{}:
		refArr = v
	default:
		me.Warn("pos=[%d]: SelectByPos: Not Array: %T", pos, me.raw)
		return me.child(nil)
	}

	containerLen := len(*refArr)

	if pos >= containerLen {
		me.Warn("pos=[%d]: SelectByPos: Overflow: %d", pos, containerLen)

		return me.child(nil)
	}

	return me.child((*refArr)[pos])
}

// Select ... func
func (me *JSONElement) Select(key1 interface{}, keys ...interface{}) *JSONElement {

	if me.IsNil() {
		me.Warn("Select: Null Object")
		return me.child(nil)
	}

	if strArr, ok := key1.([]string); ok {

		anyArr := make([]interface{}, len(strArr))
		for i, v := range strArr {
			anyArr[i] = v
		}

		key1 = anyArr
	}

	if anyArr, ok := key1.([]interface{}); ok {
		keysLen := len(keys)

		if len(anyArr)+keysLen == 0 {
			me.Warn("Select: No key")
			return me.child(nil)
		}

		if keysLen > 0 {
			anyArr = append(anyArr, keys...)
		}

		key1 = anyArr[0]
		keys = anyArr[1:]
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
func (me *JSONElement) EachMap(callback func(string, *JSONElement) (bool, error)) error {

	if me.IsNil() {
		return fmt.Errorf("EachMap: Null Object")
	}

	typedObj, ok := me.raw.(map[string]interface{})
	if !ok {
		return fmt.Errorf("EachMap: Cast: %T", me.raw)
	}

	containerLen := len(typedObj)
	if containerLen == 0 {
		return nil
	}

	keys := make([]string, containerLen)

	i := 0
	for k := range typedObj {

		keys[i] = k
		i++
	}

	sort.Strings(keys)

	for _, k := range keys {
		cont, err := callback(k, me.child(typedObj[k]))
		if err != nil {
			return fmt.Errorf("callback: %w", err)
		}

		if !cont {
			break
		}
	}

	return nil
}

// EachArray ... func
func (me *JSONElement) EachArray(callback func(int, *JSONElement) (bool, error)) error {

	if me.IsNil() {
		return fmt.Errorf("EachArray: Null Object")
	}

	var refArr *[]interface{}

	switch v := me.raw.(type) {
	case []interface{}:
		refArr = &v
	case *[]interface{}:
		refArr = v
	default:
		return fmt.Errorf("EachArray: Cast: %T", me.raw)
	}

	for i, v := range *refArr {
		cont, err := callback(i, me.child(v))

		if err != nil {
			return fmt.Errorf("callback: %w", err)
		}

		if !cont {
			break
		}
	}

	return nil
}

type walkCallbackType func([]interface{}, interface{}, interface{}) (bool, error)

func walk(argParents []interface{}, argVal interface{}, callback walkCallbackType) (bool, error) {

	var refArr *[]interface{}
	var objMap map[string]interface{}

	switch v := argVal.(type) {
	case []interface{}:
		refArr = &v
	case *[]interface{}:
		refArr = v
	case map[string]interface{}:
		objMap = v
	}

	if refArr != nil {

		for k, v := range *refArr {
			cont, err := callback(argParents, k, v)
			if err != nil {
				return false, fmt.Errorf("%v: callback: %w", k, err)
			}

			if !cont {
				return false, nil
			}

			cont, err = walk(append(argParents, k), v, callback)
			if err != nil {
				return false, fmt.Errorf("%v: walk: %w", k, err)
			}

			if !cont {
				return false, nil
			}
		}
	}

	if objMap != nil {
		for k, v := range objMap {
			cont, err := callback(argParents, k, v)
			if err != nil {
				return false, fmt.Errorf("%v: callback: %w", k, err)
			}

			if !cont {
				return false, nil
			}

			cont, err = walk(append(argParents, k), v, callback)
			if err != nil {
				return false, fmt.Errorf("%v: walk: %w", k, err)
			}

			if !cont {
				return false, nil
			}
		}
	}

	return true, nil
}

// Walk ... func
func (me *JSONElement) Walk(callback walkCallbackType) error {

	_, err := walk([]interface{}{}, me.raw, callback)

	return err
}

// ---------------------------------------------------------------------------

// AsArray ... func
func (me *JSONElement) AsArray() []*JSONElement {

	if me.IsNil() {
		me.Warn("AsArray: Null Object")
		return []*JSONElement{}
	}

	var refArr *[]interface{}

	switch v := me.raw.(type) {
	case []interface{}:
		refArr = &v
	case *[]interface{}:
		refArr = v
	default:
		me.Warn("AsArray: Cast: %T", me.raw)
		return []*JSONElement{}
	}

	arr := make([]*JSONElement, len(*refArr))

	for i, v := range *refArr {
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
