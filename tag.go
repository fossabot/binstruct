package binstruct

import (
	"github.com/pkg/errors"
	"io"
	"reflect"
	"strconv"
	"strings"
)

const (
	tagName = "bin"
	/*
		len:10,offset:20,skip:5
		len:Len,offset:Offset,skip:Skip
		func // func from struct method or FuncMap
	*/
)

const (
	tagTypeEmpty   = ""
	tagTypeIgnore  = "-"
	tagTypeFunc    = "func"
	tagTypeElement = "elem"

	tagTypeLength            = "len"
	tagTypeOffsetFromCurrent = "offset"
	tagTypeOffsetFromStart   = "offsetStart"
	tagTypeOffsetFromEnd     = "offsetEnd"
)

type tag struct {
	Type  string
	Value string

	ElemTags []tag
}

func parseTag(t string) []tag {
	var tags []tag

	for {
		var v string

		index := strings.Index(t, ",")
		switch {
		case index == -1:
			v = t
		default:
			v = t[:index]
			t = t[index+1:]
		}

		v = strings.TrimSpace(v)

		switch {
		case v == tagTypeEmpty:
			// Just skip
		case v == tagTypeIgnore:
			tags = append(tags, tag{Type: tagTypeIgnore})
		case strings.HasPrefix(v, "["):
			v = v + "," + t
			var arrBalance int
			var closeIndex int
			for {
				in := v[closeIndex:]
				idx := strings.IndexAny(in, "[]")
				closeIndex = closeIndex + idx

				switch in[idx] {
				case '[':
					arrBalance--
				case ']':
					arrBalance++
				}

				closeIndex++

				if arrBalance == 0 {
					break
				}
			}

			t = v[closeIndex:]
			v = v[1 : closeIndex-1]
			tags = append(tags, tag{Type: tagTypeElement, ElemTags: parseTag(v)})
		default:
			t := strings.Split(v, ":")

			if len(t) == 2 {
				tags = append(tags, tag{
					Type:  t[0],
					Value: t[1],
				})
			} else {
				tags = append(tags, tag{
					Type:  tagTypeFunc,
					Value: v,
				})
			}
		}

		if index == -1 {
			return tags
		}
	}

	return tags
}

type fieldOffset struct {
	Offset int64
	Whence int
}

type fieldReadData struct {
	Ignore   bool
	Length   *int64
	Offsets  []fieldOffset
	FuncName string

	ElemFieldData *fieldReadData // if type Element
}

func parseReadDataFromTags(structValue reflect.Value, tags []tag) (*fieldReadData, error) {
	parseValue := func(v string) (int64, error) {
		l, err := strconv.ParseInt(v, 10, 0)
		if err != nil {
			lenVal := structValue.FieldByName(v)
			switch lenVal.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				l = lenVal.Int()
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
				l = int64(lenVal.Uint())
			default:
				return 0, errors.New("can't get field len from " + v + " field")
			}
		}
		return l, nil
	}

	var data fieldReadData
	var err error
	for _, t := range tags {
		switch t.Type {
		case tagTypeIgnore:
			return &fieldReadData{Ignore: true}, nil

		case tagTypeLength:
			var length int64
			length, err = parseValue(t.Value)
			data.Length = &length

		case tagTypeOffsetFromCurrent:
			var offset int64
			offset, err = parseValue(t.Value)
			data.Offsets = append(data.Offsets, fieldOffset{
				Offset: offset,
				Whence: io.SeekCurrent,
			})

		case tagTypeOffsetFromStart:
			var offset int64
			offset, err = parseValue(t.Value)
			data.Offsets = append(data.Offsets, fieldOffset{
				Offset: offset,
				Whence: io.SeekStart,
			})

		case tagTypeOffsetFromEnd:
			var offset int64
			offset, err = parseValue(t.Value)
			data.Offsets = append(data.Offsets, fieldOffset{
				Offset: offset,
				Whence: io.SeekEnd,
			})

		case tagTypeFunc:
			data.FuncName = t.Value

		case tagTypeElement:
			data.ElemFieldData, err = parseReadDataFromTags(structValue, t.ElemTags)
		}

		if err != nil {
			return nil, err
		}
	}

	return &data, nil
}
