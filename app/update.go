package app

import (
	"fmt"
	"reflect"

	"github.com/golang/protobuf/proto"
)

type UpdateOptions struct {
	// Operation Context
	Context
	// Optional. Partial fields set to be updated.
	// Otherwise, it means a full object update.
	Fields []string
}


func Sanitize(src proto.Message, fields ...string) {
	in := reflect.ValueOf(src)
	st := proto.GetProperties(in.Type())

	var fv reflect.Value
	for _, fname := range fields {

		fv = reflect.Value{}
		for _, prop := range st.Prop {
			if prop.OrigName == fname {
				fv = in.FieldByName(prop.Name)
				break
			}
		}

		if fv.IsValid() {
			fv.Set(reflect.Zero(fv.Type()))
		}
	}
}

func Merge(dst, src interface{}, fields ...string) {
	in := reflect.ValueOf(src)
	out := reflect.ValueOf(dst)
	if out.IsNil() {
		panic(`merge: <nil> destination`)
	}
	if in.Type() != out.Type() {
		panic(fmt.Sprintf("merge: types mismatch (%T != %T)", dst, src))
	}
	if in.IsNil() {
		return // Merge from nil src is a noop
	}
	// if m, ok := dst.(generatedMerger); ok {
	// 	m.XXX_Merge(src)
	// 	return
	// }
	mergeStruct(out.Elem(), in.Elem(), fields)
}

func MergeProto(dst, src proto.Message, fields ...string) {
	// if m, ok := dst.(proto.Merger); ok {
	// 	m.Merge(src)
	// 	return
	// }

	in := reflect.ValueOf(src)
	out := reflect.ValueOf(dst)
	if out.IsNil() {
		panic("merge: <nil> destination")
	}
	if in.Type() != out.Type() {
		panic(fmt.Sprintf("merge: types mismatch (%T != %T)", dst, src))
	}
	if in.IsNil() {
		return // Merge from nil src is a noop
	}
	// if m, ok := dst.(generatedMerger); ok {
	// 	m.XXX_Merge(src)
	// 	return
	// }
	mergeStruct(out.Elem(), in.Elem(), fields)
}

/*
func mergeStruct(dst, src reflect.Value, fields []string) {
	props := proto.GetProperties(src.Type())

	Next:
	for _, fname := range fields {
		for _, fprop := range props.Prop {
			if fname == fprop.OrigName {
				// if fpath == basePath + "." + fprop.OrigName {
				// MATCH:
				field, _ := dst.Type().FieldByName(fprop.Name)
				dst.FieldByIndex(field.Index).Set(
					src.FieldByIndex(field.Index),
				)
				continue Next
			}
		}
		// PANIC if field not found !
		// panic(errors.InvalidArgumentError(
		// 	"app.update.field.not_found",
		// 	"update: objclass=%s attribute=%s not found",
		// 	 src.Type().Name(), fname,
		// ))
		panic("update:" +
			" objclass=" + src.Type().Name() +
			" attribute=" + fname + " not found",
		)
	}
}*/

func mergeStruct(dst, src reflect.Value, fields []string) {

	i, n := 0, len(fields)
	props := proto.GetProperties(src.Type())

	Next:
	for _, att := range props.Prop {
		for i = 0; i < n && fields[i] != att.OrigName; i++ {
			// LOOKUP: requested field(s) index !
		}
		if n != 0 && i == n {
			// OMIT: fields specified but NOT this att
			continue Next
		}
		// UPDATE:
		field, _ := dst.Type().FieldByName(att.Name)
		dst.FieldByIndex(field.Index).Set(
			src.FieldByIndex(field.Index),
		)
	}

	// NOTE: Unknown fields will NOT panic; just omitted!
}