package dynajson

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAll(t *testing.T) {
	TestWrite1(t)
	TestRead1(t)
	TestRead2(t)
}

func TestWrite1(t *testing.T) {

	assert := assert.New(t)

	root := NewAsMap()
	root.WarnHandler = func(me *JSONElement, message string, where string, line int) {
		fmt.Fprintf(os.Stderr, "Warn(%d): %s(%d): %s\n", me.Level, where, line, message)
	}

	m1, err := root.PutEmptyMap("m1")
	assert.Nil(err)

	m1.Put("m1s1", "str1")
	m1.Put("m1i1", 101)
	m1.Put("m1i2", 999999)
	m1.Put("m1a1", 1, 2, 3, "a", "b", "c")

	err = root.Select("m1").Delete("m1i2")
	assert.Nil(err)

	err = root.Select("m1").Select("m1a1").Delete(3)
	assert.Nil(err)

	m1a1 := root.Select("m1").Select("m1a1")
	m1a1.Append(0.5)

	root.Put("s1", `"`)
	root.Put("a1", `\`, `"`)

	root.PutEmptyMap("m2")
	root.Select("m2").Put("m2s1", "str2")
	m2 := root.Select("m2")
	m2.Put("m2i1", 201)

	sum := 0
	root.Select("m1").Select("m1a1").EachArray(func(i int, elm *JSONElement) {
		sum++
	})

	assert.Equal(4, root.Count())
	assert.Equal(6, m1.Select("m1a1").Count())
	assert.Equal(6, sum)
	assert.Equal(6, len(root.Select("m1").Select("m1a1").AsArray()))
	assert.Equal("str2", m2.Select("m2s1").AsString())
	assert.Equal(3, m1.Select("m1a1").Select(2).AsInt())
	assert.Equal(1, root.Select("m2").Select("m2i1").Count()) // warn OK

	fmt.Println(root.String())
}

func TestRead1(t *testing.T) {

	assert := assert.New(t)

	root, err := NewByPath("https://petstore.swagger.io/v2/swagger.json")
	assert.Nil(err)

	if err != nil {
		return
	}

	root.WarnHandler = func(me *JSONElement, message string, where string, line int) {
		fmt.Fprintf(os.Stderr, "Warn(%d): %s(%d): %s\n", me.Level, where, line, message)
	}

	tags := root.Select("tags")

	sum := 0
	root.Select("schemes").EachArray(func(i int, val *JSONElement) {
		sum++
	})

	schemes := root.Select("schemes").AsArray()
	definitions := root.Select("definitions")

	keys1 := []string{}
	definitions.Select("ApiResponse").Select("properties").EachMap(func(key string, val *JSONElement) {
		keys1 = append(keys1, key)
	})

	properties := root.Select("definitions").Select("ApiResponse").Select("properties")
	keys2 := properties.Keys()

	assert.Equal("2.0", root.Select("swagger").AsString())
	assert.Equal("store", tags.Select(1).Select("name").AsString())
	assert.Equal(2, sum)
	assert.Equal(2, len(schemes))
	assert.True(len(keys1) == len(keys2))
}

func currentDir() (dir string) {
	_, path, _, _ := runtime.Caller(1)
	dir = filepath.Dir(path)
	return
}

func TestRead2(t *testing.T) {

	assert := assert.New(t)

	jsonPath := filepath.Join(currentDir(), "testdata", "read2.json")

	root, err := NewByPath(jsonPath)
	assert.Nil(err)

	if err != nil {
		return
	}

	root.WarnHandler = func(me *JSONElement, message string, where string, line int) {
		fmt.Fprintf(os.Stderr, "Warn(%d): %s(%d): %s\n", me.Level, where, line, message)
	}

	glossDiv := root.Select("glossary").Select("GlossDiv")

	seeAlso := glossDiv.Select("GlossList").Select("GlossEntry").Select("GlossDef").Select("GlossSeeAlso").String()

	bytes, err := ioutil.ReadFile(jsonPath)
	assert.Nil(err)

	root2, err := NewByBytes(bytes)
	assert.Nil(err)

	root2.WarnHandler = func(me *JSONElement, message string, where string, line int) {
		fmt.Fprintf(os.Stderr, "Warn(%d): %s(%d): %s\n", me.Level, where, line, message)
	}

	seeAlso2 := root2.Select("glossary").Select("GlossDiv").Select("GlossList").Select("GlossEntry").Select("GlossDef").Select("GlossSeeAlso").String()

	assert.Equal(glossDiv.Select("title").AsString(), "S")
	assert.Equal(seeAlso, seeAlso2)
}

func TestReadonly(t *testing.T) {

	assert := assert.New(t)

	root, err := NewByString(`{"str":"abc", "int": 123, "arr":["a", "b", 1, 2], "map":{"mapstr": "ABC", "mapint":455}}`)
	assert.Nil(err)

	assert.Equal("abc", root.Select("str").AsString())

	err = root.Put("str2", "def")
	assert.Nil(err)

	root.Readonly = true
	err = root.Put("str3", "DEF")
	assert.NotNil(err)

	err = root.Delete("int")
	assert.NotNil(err)

	sub := root.Select("map")
	err = sub.Put("mapstr2", "DEF")
	assert.NotNil(err)
}
