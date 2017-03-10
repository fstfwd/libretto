// Copyright 2013 Apcera Inc. All rights reserved.

package envmap

import (
	"fmt"
	"os"
)

// Provides a simple storage layer for environment like variables.
type EnvMap struct {
	Env     map[string]string
	Parent  *EnvMap
	Flatten bool
}

func NewEnvMap() (r *EnvMap) {
	r = new(EnvMap)
	r.Env = make(map[string]string, 0)
	r.Flatten = true
	return r
}

// FlattenMap when set to false will not flatten the
// results of an EnvMap.
func (e *EnvMap) FlattenMap(flatMap bool) {
	e.Flatten = flatMap
}

func (e *EnvMap) Set(key, value string) {
	if prev, ok := e.Env[key]; ok == true {
		resolve := func(s string) string {
			if s == key {
				return prev
			}
			return "$" + key
		}
		e.Env[key] = os.Expand(value, resolve)
	} else {
		e.Env[key] = value
	}
}

func (e *EnvMap) get(
	key string, top *EnvMap, processQueue map[string]*EnvMap,
	cache map[string]string,
) (string, bool) {
	resolve := func(s string) string {
		if value, ok := cache[s]; ok == true {
			return value
		}
		if last, ok := processQueue[s]; ok == true {
			// If this is the last element in this environment map
			// then we return ""
			if last == nil {
				return ""
			}
			processQueue[s] = last.Parent
			r, _ := last.get(s, top, processQueue, cache)
			return r
		}
		processQueue[s] = top
		r, _ := top.get(s, top, processQueue, cache)
		return r
	}

	for e != nil {
		if value, ok := e.Env[key]; ok == true {
			processQueue[key] = e.Parent
			s := os.Expand(value, resolve)
			delete(processQueue, key)
			return s, true
		}
		e = e.Parent
	}
	return "", false
}

func (e *EnvMap) Get(key string) (string, bool) {
	// This is used to ensure that we do not recurse forever while
	// attempting to get a variable.
	processQueue := make(map[string]*EnvMap, 10)
	cache := make(map[string]string, 1)
	return e.get(key, e, processQueue, cache)
}

func (e *EnvMap) GetRaw(key string) (string, bool) {
	for e != nil {
		if value, ok := e.Env[key]; ok == true {
			return value, true
		}
		e = e.Parent
	}
	return "", false
}

func (e *EnvMap) Map() map[string]string {
	cache := make(map[string]string, len(e.Env))
	processQueue := make(map[string]*EnvMap, 10)

	for p := e; p != nil; p = p.Parent {
		for k := range p.Env {
			if _, ok := cache[k]; ok == false {
				if e.Flatten {
					cache[k], _ = e.get(k, e, processQueue, cache)
				} else {
					cache[k], _ = e.GetRaw(k)
				}
			}
		}
	}

	return cache
}

func (e *EnvMap) Strings() []string {
	m := e.Map()
	r := make([]string, 0, len(m))
	for k, v := range m {
		r = append(r, fmt.Sprintf("%s=%s", k, v))
	}
	return r
}

// Keys returns an array of the keys of the environment map.
func (e *EnvMap) Keys() []string {
	var keys []string
	for k, _ := range e.Map() {
		keys = append(keys, k)
	}

	return keys
}

func (e *EnvMap) NewChild() *EnvMap {
	return &EnvMap{
		Env:     make(map[string]string, 0),
		Parent:  e,
		Flatten: true,
	}
}
