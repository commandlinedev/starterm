// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/commandlinedev/starterm/pkg/service/blockservice"
	"github.com/commandlinedev/starterm/pkg/service/clientservice"
	"github.com/commandlinedev/starterm/pkg/service/objectservice"
	"github.com/commandlinedev/starterm/pkg/service/userinputservice"
	"github.com/commandlinedev/starterm/pkg/service/windowservice"
	"github.com/commandlinedev/starterm/pkg/service/workspaceservice"
	"github.com/commandlinedev/starterm/pkg/starobj"
	"github.com/commandlinedev/starterm/pkg/tsgen/tsgenmeta"
	"github.com/commandlinedev/starterm/pkg/util/utilfn"
	"github.com/commandlinedev/starterm/pkg/web/webcmd"
)

var ServiceMap = map[string]any{
	"block":     blockservice.BlockServiceInstance,
	"object":    &objectservice.ObjectService{},
	"client":    &clientservice.ClientService{},
	"window":    &windowservice.WindowService{},
	"workspace": &workspaceservice.WorkspaceService{},
	"userinput": &userinputservice.UserInputService{},
}

var contextRType = reflect.TypeOf((*context.Context)(nil)).Elem()
var errorRType = reflect.TypeOf((*error)(nil)).Elem()
var updatesRType = reflect.TypeOf(([]starobj.StarObjUpdate{}))
var starObjRType = reflect.TypeOf((*starobj.StarObj)(nil)).Elem()
var starObjSliceRType = reflect.TypeOf([]starobj.StarObj{})
var starObjMapRType = reflect.TypeOf(map[string]starobj.StarObj{})
var methodMetaRType = reflect.TypeOf(tsgenmeta.MethodMeta{})
var starObjUpdateRType = reflect.TypeOf(starobj.StarObjUpdate{})
var uiContextRType = reflect.TypeOf((*starobj.UIContext)(nil)).Elem()
var wsCommandRType = reflect.TypeOf((*webcmd.WSCommandType)(nil)).Elem()
var orefRType = reflect.TypeOf((*starobj.ORef)(nil)).Elem()

type WebCallType struct {
	Service   string             `json:"service"`
	Method    string             `json:"method"`
	UIContext *starobj.UIContext `json:"uicontext,omitempty"`
	Args      []any              `json:"args"`
}

type WebReturnType struct {
	Success bool                    `json:"success,omitempty"`
	Error   string                  `json:"error,omitempty"`
	Data    any                     `json:"data,omitempty"`
	Updates []starobj.StarObjUpdate `json:"updates,omitempty"`
}

func convertNumber(argType reflect.Type, jsonArg float64) (any, error) {
	switch argType.Kind() {
	case reflect.Int:
		return int(jsonArg), nil
	case reflect.Int8:
		return int8(jsonArg), nil
	case reflect.Int16:
		return int16(jsonArg), nil
	case reflect.Int32:
		return int32(jsonArg), nil
	case reflect.Int64:
		return int64(jsonArg), nil
	case reflect.Uint:
		return uint(jsonArg), nil
	case reflect.Uint8:
		return uint8(jsonArg), nil
	case reflect.Uint16:
		return uint16(jsonArg), nil
	case reflect.Uint32:
		return uint32(jsonArg), nil
	case reflect.Uint64:
		return uint64(jsonArg), nil
	case reflect.Float32:
		return float32(jsonArg), nil
	case reflect.Float64:
		return jsonArg, nil
	}
	return nil, fmt.Errorf("invalid number type %s", argType)
}

func convertComplex(argType reflect.Type, jsonArg any) (any, error) {
	nativeArgVal := reflect.New(argType)
	err := utilfn.DoMapStructure(nativeArgVal.Interface(), jsonArg)
	if err != nil {
		return nil, err
	}
	return nativeArgVal.Elem().Interface(), nil
}

func isSpecialStarArgType(argType reflect.Type) bool {
	return argType == starObjRType || argType == starObjSliceRType || argType == starObjMapRType || argType == wsCommandRType
}

func convertWSCommand(argType reflect.Type, jsonArg any) (any, error) {
	if _, ok := jsonArg.(map[string]any); !ok {
		return nil, fmt.Errorf("cannot convert %T to %s", jsonArg, argType)
	}
	cmd, err := webcmd.ParseWSCommandMap(jsonArg.(map[string]any))
	if err != nil {
		return nil, fmt.Errorf("error parsing command map: %w", err)
	}
	return cmd, nil
}

func convertSpecial(argType reflect.Type, jsonArg any) (any, error) {
	jsonType := reflect.TypeOf(jsonArg)
	if argType == orefRType {
		if jsonType.Kind() != reflect.String {
			return nil, fmt.Errorf("cannot convert %T to %s", jsonArg, argType)
		}
		oref, err := starobj.ParseORef(jsonArg.(string))
		if err != nil {
			return nil, fmt.Errorf("invalid oref string: %v", err)
		}
		return oref, nil
	} else if argType == wsCommandRType {
		return convertWSCommand(argType, jsonArg)
	} else if argType == starObjRType {
		if jsonType.Kind() != reflect.Map {
			return nil, fmt.Errorf("cannot convert %T to %s", jsonArg, argType)
		}
		return starobj.FromJsonMap(jsonArg.(map[string]any))
	} else if argType == starObjSliceRType {
		if jsonType.Kind() != reflect.Slice {
			return nil, fmt.Errorf("cannot convert %T to %s", jsonArg, argType)
		}
		sliceArg := jsonArg.([]any)
		nativeSlice := make([]starobj.StarObj, len(sliceArg))
		for idx, elem := range sliceArg {
			elemMap, ok := elem.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("cannot convert %T to %s (idx %d is not a map, is %T)", jsonArg, starObjSliceRType, idx, elem)
			}
			nativeObj, err := starobj.FromJsonMap(elemMap)
			if err != nil {
				return nil, fmt.Errorf("cannot convert %T to %s (idx %d) error: %v", jsonArg, starObjSliceRType, idx, err)
			}
			nativeSlice[idx] = nativeObj
		}
		return nativeSlice, nil
	} else if argType == starObjMapRType {
		if jsonType.Kind() != reflect.Map {
			return nil, fmt.Errorf("cannot convert %T to %s", jsonArg, argType)
		}
		mapArg := jsonArg.(map[string]any)
		nativeMap := make(map[string]starobj.StarObj)
		for key, elem := range mapArg {
			elemMap, ok := elem.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("cannot convert %T to %s (key %s is not a map, is %T)", jsonArg, starObjMapRType, key, elem)
			}
			nativeObj, err := starobj.FromJsonMap(elemMap)
			if err != nil {
				return nil, fmt.Errorf("cannot convert %T to %s (key %s) error: %v", jsonArg, starObjMapRType, key, err)
			}
			nativeMap[key] = nativeObj
		}
		return nativeMap, nil
	} else {
		return nil, fmt.Errorf("invalid special star argument type %s", argType)
	}
}

func convertSpecialForReturn(argType reflect.Type, nativeArg any) (any, error) {
	if argType == starObjRType {
		return starobj.ToJsonMap(nativeArg.(starobj.StarObj))
	} else if argType == starObjSliceRType {
		nativeSlice := nativeArg.([]starobj.StarObj)
		jsonSlice := make([]map[string]any, len(nativeSlice))
		for idx, elem := range nativeSlice {
			elemMap, err := starobj.ToJsonMap(elem)
			if err != nil {
				return nil, err
			}
			jsonSlice[idx] = elemMap
		}
		return jsonSlice, nil
	} else if argType == starObjMapRType {
		nativeMap := nativeArg.(map[string]starobj.StarObj)
		jsonMap := make(map[string]map[string]any)
		for key, elem := range nativeMap {
			elemMap, err := starobj.ToJsonMap(elem)
			if err != nil {
				return nil, err
			}
			jsonMap[key] = elemMap
		}
		return jsonMap, nil
	} else {
		return nil, fmt.Errorf("invalid special star argument type %s", argType)
	}
}

func convertArgument(argType reflect.Type, jsonArg any) (any, error) {
	if jsonArg == nil {
		return reflect.Zero(argType).Interface(), nil
	}
	if isSpecialStarArgType(argType) {
		return convertSpecial(argType, jsonArg)
	}
	jsonType := reflect.TypeOf(jsonArg)
	switch argType.Kind() {
	case reflect.String:
		if jsonType.Kind() == reflect.String {
			return jsonArg, nil
		}
		return nil, fmt.Errorf("cannot convert %T to %s", jsonArg, argType)

	case reflect.Bool:
		if jsonType.Kind() == reflect.Bool {
			return jsonArg, nil
		}
		return nil, fmt.Errorf("cannot convert %T to %s", jsonArg, argType)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		if jsonType.Kind() == reflect.Float64 {
			return convertNumber(argType, jsonArg.(float64))
		}
		return nil, fmt.Errorf("cannot convert %T to %s", jsonArg, argType)

	case reflect.Map:
		if argType.Key().Kind() != reflect.String {
			return nil, fmt.Errorf("invalid map key type %s", argType.Key())
		}
		if jsonType.Kind() != reflect.Map {
			return nil, fmt.Errorf("cannot convert %T to %s", jsonArg, argType)
		}
		return convertComplex(argType, jsonArg)

	case reflect.Slice:
		if jsonType.Kind() != reflect.Slice {
			return nil, fmt.Errorf("cannot convert %T to %s", jsonArg, argType)
		}
		return convertComplex(argType, jsonArg)

	case reflect.Struct:
		if jsonType.Kind() != reflect.Map {
			return nil, fmt.Errorf("cannot convert %T to %s", jsonArg, argType)
		}
		return convertComplex(argType, jsonArg)

	case reflect.Ptr:
		if argType.Elem().Kind() != reflect.Struct {
			return nil, fmt.Errorf("invalid pointer type %s", argType)
		}
		if jsonType.Kind() != reflect.Map {
			return nil, fmt.Errorf("cannot convert %T to %s", jsonArg, argType)
		}
		return convertComplex(argType, jsonArg)

	default:
		return nil, fmt.Errorf("invalid argument type %s", argType)
	}
}

func isNilable(val reflect.Value) bool {
	switch val.Kind() {
	case reflect.Ptr, reflect.Slice, reflect.Map, reflect.Interface, reflect.Chan, reflect.Func:
		return true
	}
	return false

}

func convertReturnValues(rtnVals []reflect.Value) *WebReturnType {
	rtn := &WebReturnType{}
	if len(rtnVals) == 0 {
		return rtn
	}
	for _, val := range rtnVals {
		if isNilable(val) && val.IsNil() {
			continue
		}
		valType := val.Type()
		if valType == errorRType {
			rtn.Error = val.Interface().(error).Error()
			continue
		}
		if valType == updatesRType {
			// has a special MarshalJSON method
			rtn.Updates = val.Interface().([]starobj.StarObjUpdate)
			continue
		}
		if isSpecialStarArgType(valType) {
			jsonVal, err := convertSpecialForReturn(valType, val.Interface())
			if err != nil {
				rtn.Error = fmt.Errorf("cannot convert special return value: %v", err).Error()
				continue
			}
			rtn.Data = jsonVal
			continue
		}
		rtn.Data = val.Interface()
	}
	if rtn.Error == "" {
		rtn.Success = true
	}
	return rtn
}

func webErrorRtn(err error) *WebReturnType {
	return &WebReturnType{
		Error: err.Error(),
	}
}

func CallService(ctx context.Context, webCall WebCallType) *WebReturnType {
	svcObj := ServiceMap[webCall.Service]
	if svcObj == nil {
		return webErrorRtn(fmt.Errorf("invalid service: %q", webCall.Service))
	}
	method := reflect.ValueOf(svcObj).MethodByName(webCall.Method)
	if !method.IsValid() {
		return webErrorRtn(fmt.Errorf("invalid method: %s.%s", webCall.Service, webCall.Method))
	}
	var valueArgs []reflect.Value
	argIdx := 0
	for idx := 0; idx < method.Type().NumIn(); idx++ {
		argType := method.Type().In(idx)
		if idx == 0 && argType == contextRType {
			valueArgs = append(valueArgs, reflect.ValueOf(ctx))
			continue
		}
		if argType == uiContextRType {
			if webCall.UIContext == nil {
				return webErrorRtn(fmt.Errorf("missing UIContext for %s.%s", webCall.Service, webCall.Method))
			}
			valueArgs = append(valueArgs, reflect.ValueOf(*webCall.UIContext))
			continue
		}
		if argIdx >= len(webCall.Args) {
			return webErrorRtn(fmt.Errorf("not enough arguments passed %s.%s idx:%d (type %T)", webCall.Service, webCall.Method, idx, argType))
		}
		nativeArg, err := convertArgument(argType, webCall.Args[argIdx])
		if err != nil {
			return webErrorRtn(fmt.Errorf("cannot convert argument %s.%s type:%T idx:%d error:%v", webCall.Service, webCall.Method, argType, idx, err))
		}
		valueArgs = append(valueArgs, reflect.ValueOf(nativeArg))
		argIdx++
	}
	retValArr := method.Call(valueArgs)
	return convertReturnValues(retValArr)
}

// ValidateServiceArg validates the argument type for a service method
// does not allow interfaces (and the obvious invalid types)
// arguments + return values have special handling for star objects
func baseValidateServiceArg(argType reflect.Type) error {
	if argType == starObjUpdateRType {
		// has special MarshalJSON method, so it is safe
		return nil
	}
	switch argType.Kind() {
	case reflect.Ptr, reflect.Slice, reflect.Array:
		return baseValidateServiceArg(argType.Elem())
	case reflect.Map:
		if argType.Key().Kind() != reflect.String {
			return fmt.Errorf("invalid map key type %s", argType.Key())
		}
		return baseValidateServiceArg(argType.Elem())
	case reflect.Struct:
		for idx := 0; idx < argType.NumField(); idx++ {
			if err := baseValidateServiceArg(argType.Field(idx).Type); err != nil {
				return err
			}
		}
	case reflect.Interface:
		return fmt.Errorf("invalid argument type %s: contains interface", argType)

	case reflect.Chan, reflect.Func, reflect.Complex128, reflect.Complex64, reflect.Invalid, reflect.Uintptr, reflect.UnsafePointer:
		return fmt.Errorf("invalid argument type %s", argType)
	}
	return nil
}

func validateMethodReturnArg(retType reflect.Type) error {
	// specifically allow starobj.StarObj, []starobj.StarObj, map[string]starobj.StarObj, and error
	if isSpecialStarArgType(retType) || retType == errorRType {
		return nil
	}
	return baseValidateServiceArg(retType)
}

func validateMethodArg(argType reflect.Type) error {
	// specifically allow starobj.StarObj, []starobj.StarObj, map[string]starobj.StarObj, and context.Context
	if isSpecialStarArgType(argType) || argType == contextRType {
		return nil
	}
	return baseValidateServiceArg(argType)
}

func validateServiceMethod(service string, method reflect.Method) error {
	for idx := 0; idx < method.Type.NumOut(); idx++ {
		if err := validateMethodReturnArg(method.Type.Out(idx)); err != nil {
			return fmt.Errorf("invalid return type %s.%s %s: %v", service, method.Name, method.Type.Out(idx), err)
		}
	}
	for idx := 1; idx < method.Type.NumIn(); idx++ {
		// skip the first argument which is the receiver
		if err := validateMethodArg(method.Type.In(idx)); err != nil {
			return fmt.Errorf("invalid argument type %s.%s %s: %v", service, method.Name, method.Type.In(idx), err)
		}
	}
	return nil
}

func validateServiceMetaMethod(service string, method reflect.Method) error {
	if method.Type.NumIn() != 1 {
		return fmt.Errorf("invalid number of arguments %s.%s: got:%d, expected just the receiver", service, method.Name, method.Type.NumIn())
	}
	if method.Type.NumOut() != 1 && method.Type.Out(0) != methodMetaRType {
		return fmt.Errorf("invalid return type %s.%s: got:%s, expected servicemeta.MethodMeta", service, method.Name, method.Type.Out(0))
	}
	return nil
}

func ValidateService(serviceName string, svcObj any) error {
	svcType := reflect.TypeOf(svcObj)
	if svcType.Kind() != reflect.Ptr {
		return fmt.Errorf("service object %q must be a pointer", serviceName)
	}
	svcType = svcType.Elem()
	if svcType.Kind() != reflect.Struct {
		return fmt.Errorf("service object %q must be a ptr to struct", serviceName)
	}
	for idx := 0; idx < svcType.NumMethod(); idx++ {
		method := svcType.Method(idx)
		if strings.HasSuffix(method.Name, "_Meta") {
			err := validateServiceMetaMethod(serviceName, method)
			if err != nil {
				return err
			}
		}
		if err := validateServiceMethod(serviceName, method); err != nil {
			return err
		}
	}
	return nil
}

func ValidateServiceMap() error {
	for svcName, svcObj := range ServiceMap {
		if err := ValidateService(svcName, svcObj); err != nil {
			return err
		}
	}
	return nil
}
