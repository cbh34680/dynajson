# dynajson

## Description

An implementation of JSON Utility for the Go programming language.

## Installation

```
go get github.com/cbh34680/dynajson
```

## Usage

### Example

```go

package main

import (
    "fmt"
    "github.com/cbh34680/dynajson"
)

func readSample() error {

    url := "https://petstore.swagger.io/v2/swagger.json"
    root, err := dynajson.NewByPath(url)
    if err != nil {
        return fmt.Errorf("NewByPath: %s: %w", url, err)
    }
    
    fmt.Println(root.Select("swagger").AsString())
    
    info := root.Select("info")
    fmt.Println(info.Select("title").AsString())
    
    root.Select("definitions").EachMap(func(key string, elm *dynajson.JSONElement) {
        fmt.Printf("%s %s\n", key, elm.Select("type"))
    })
    
    return nil
}

func writeSample() error {

    root := dynajson.NewRootAsMap()

    root.Put("str", "abc")
    root.Put("arr", 10, "a", 10.1)

    sub, err := root.PutEmptyMap("map")
    if err != nil {
        return fmt.Errorf("PutEmptyMap: %w", err)
    }

    sub.Put("int", 100)
    arr, _ := sub.PutEmptyArray("arr")

    arr.Append(20)
    arr.Append("b", 20.2)

    fmt.Println(root) // `{"str": "abc", "arr": [10, "a", 10.1], "map": {"int": 100, "arr": [20, "b", 20.2]}}`

    return nil
}

func readSample2() error {

    orig := map[string]interface{}{}
    orig["str"] = "abc"

    arrObj := []interface{}{10, "a", 10.1}
    orig["arr"] = arrObj

    mapObj := map[string]interface{}{}
    mapObj["int"] = 100
    orig["map"] = mapObj

    root := dynajson.New(orig)
    fmt.Println(root) // `{"str": "abc", "arr": [10, "a", 10.1], "map": {"int": 100}}`

    //
    bytes, _ := json.Marshal(orig)
    fmt.Println(string(bytes))

    json.Unmarshal(bytes, &orig)

    root, _ = dynajson.NewByBytes(bytes)
    fmt.Println(root) // `{"str": "abc", "arr": [10, "a", 10.1], "map": {"int": 100}}`

    return nil
}

func main() {

    readSample()
    writeSample()
    readSample2()
}


```
