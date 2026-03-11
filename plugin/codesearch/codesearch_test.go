package codesearch

import (
	"strings"
	"testing"
)

// --- RegexExtractor Tests ---

func TestRegexExtractor_Go(t *testing.T) {
	source := []byte(`package main

import "fmt"

const MaxRetries = 3

var globalCounter int

type User struct {
	Name string
	Age  int
}

type Stringer interface {
	String() string
}

type UserID int

func NewUser(name string) *User {
	return &User{Name: name}
}

func (u *User) String() string {
	return u.Name
}
`)

	ext := NewRegexExtractor()
	symbols, err := ext.Extract(source, "go")
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]SymbolKind{
		"main":          SymbolPackage,
		"MaxRetries":    SymbolConstant,
		"globalCounter": SymbolVariable,
		"User":          SymbolStruct,
		"Stringer":      SymbolInterface,
		"UserID":        SymbolType,
		"NewUser":       SymbolFunction,
		"String":        SymbolMethod,
	}

	found := make(map[string]SymbolKind)
	for _, sym := range symbols {
		found[sym.Name] = sym.Kind
	}

	for name, kind := range expected {
		if got, ok := found[name]; !ok {
			t.Errorf("expected symbol %q not found", name)
		} else if got != kind {
			t.Errorf("symbol %q: expected kind %q, got %q", name, kind, got)
		}
	}
}

func TestRegexExtractor_Python(t *testing.T) {
	source := []byte(`import os
from pathlib import Path

class Animal:
    def __init__(self, name):
        self.name = name

    def speak(self):
        pass

def create_animal(name):
    return Animal(name)

MAX_SIZE = 100
`)

	ext := NewRegexExtractor()
	symbols, err := ext.Extract(source, "python")
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]SymbolKind{
		"os":            SymbolImport,
		"pathlib":       SymbolImport,
		"Animal":        SymbolClass,
		"__init__":      SymbolMethod,
		"speak":         SymbolMethod,
		"create_animal": SymbolFunction,
		"MAX_SIZE":      SymbolVariable,
	}

	found := make(map[string]SymbolKind)
	for _, sym := range symbols {
		found[sym.Name] = sym.Kind
	}

	for name, kind := range expected {
		if got, ok := found[name]; !ok {
			t.Errorf("expected symbol %q not found", name)
		} else if got != kind {
			t.Errorf("symbol %q: expected kind %q, got %q", name, kind, got)
		}
	}
}

func TestRegexExtractor_JavaScript(t *testing.T) {
	source := []byte(`import React from 'react';

export class UserComponent {
    render() {
        return null;
    }
}

export function fetchUsers() {
    return [];
}

export const API_URL = "https://api.example.com";
const helper = (x) => x + 1;
let counter = 0;
`)

	ext := NewRegexExtractor()
	symbols, err := ext.Extract(source, "javascript")
	if err != nil {
		t.Fatal(err)
	}

	found := make(map[string]SymbolKind)
	for _, sym := range symbols {
		found[sym.Name] = sym.Kind
	}

	if _, ok := found["UserComponent"]; !ok {
		t.Error("expected UserComponent class")
	}
	if _, ok := found["fetchUsers"]; !ok {
		t.Error("expected fetchUsers function")
	}
}

func TestRegexExtractor_Rust(t *testing.T) {
	source := []byte(`pub mod utils;

use std::collections::HashMap;

pub struct Config {
    name: String,
}

pub enum Color {
    Red,
    Green,
    Blue,
}

pub trait Drawable {
    fn draw(&self);
}

pub fn initialize() {
    println!("init");
}

impl Config {
    pub fn new(name: &str) -> Self {
        Config { name: name.to_string() }
    }
}

pub const MAX_SIZE: usize = 1024;

pub type Result<T> = std::result::Result<T, Error>;
`)

	ext := NewRegexExtractor()
	symbols, err := ext.Extract(source, "rust")
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]SymbolKind{
		"utils":      SymbolModule,
		"Config":     SymbolStruct,
		"Color":      SymbolEnum,
		"Drawable":   SymbolTrait,
		"initialize": SymbolFunction,
		"MAX_SIZE":   SymbolConstant,
	}

	found := make(map[string]SymbolKind)
	for _, sym := range symbols {
		found[sym.Name] = sym.Kind
	}

	for name, kind := range expected {
		if got, ok := found[name]; !ok {
			t.Errorf("expected symbol %q not found", name)
		} else if got != kind {
			t.Errorf("symbol %q: expected kind %q, got %q", name, kind, got)
		}
	}
}

func TestRegexExtractor_Java(t *testing.T) {
	source := []byte(`package com.example;

import java.util.List;

public class UserService {
    private final UserRepo repo;

    public UserService(UserRepo repo) {
        this.repo = repo;
    }

    public List<User> getUsers() {
        return repo.findAll();
    }
}

public interface UserRepo {
    List<User> findAll();
}

public enum Role {
    ADMIN, USER, GUEST
}
`)

	ext := NewRegexExtractor()
	symbols, err := ext.Extract(source, "java")
	if err != nil {
		t.Fatal(err)
	}

	found := make(map[string]SymbolKind)
	for _, sym := range symbols {
		found[sym.Name] = sym.Kind
	}

	if _, ok := found["UserService"]; !ok {
		t.Error("expected UserService class")
	}
	if _, ok := found["UserRepo"]; !ok {
		t.Error("expected UserRepo interface")
	}
	if _, ok := found["Role"]; !ok {
		t.Error("expected Role enum")
	}
}

func TestRegexExtractor_LanguageAliases(t *testing.T) {
	ext := NewRegexExtractor()
	source := []byte(`func main() {}`)

	for _, alias := range []string{"go", "golang"} {
		symbols, err := ext.Extract(source, alias)
		if err != nil {
			t.Fatalf("alias %q: %v", alias, err)
		}
		if len(symbols) == 0 {
			t.Errorf("alias %q: expected symbols", alias)
		}
	}
}

func TestRegexExtractor_UnknownLanguage(t *testing.T) {
	ext := NewRegexExtractor()
	source := []byte(`func main() {}`)

	// Should try all languages and return best match.
	symbols, err := ext.Extract(source, "unknown")
	if err != nil {
		t.Fatal(err)
	}
	// Should find at least something from one of the language patterns.
	if len(symbols) == 0 {
		t.Error("expected some symbols even for unknown language")
	}
}

func TestRegexExtractor_CustomLanguage(t *testing.T) {
	ext := NewRegexExtractor()
	if err := ext.AddLanguage("mylang", []PatternDef{
		{Pattern: `^proc\s+(\w+)`, Kind: SymbolFunction, NameGroup: 1, ScopeGroup: -1},
		{Pattern: `^record\s+(\w+)`, Kind: SymbolStruct, NameGroup: 1, ScopeGroup: -1},
	}); err != nil {
		t.Fatal(err)
	}

	source := []byte("proc doStuff\nrecord MyData\n")
	symbols, err := ext.Extract(source, "mylang")
	if err != nil {
		t.Fatal(err)
	}
	if len(symbols) != 2 {
		t.Fatalf("expected 2 symbols, got %d", len(symbols))
	}
	if symbols[0].Name != "doStuff" || symbols[0].Kind != SymbolFunction {
		t.Errorf("unexpected first symbol: %+v", symbols[0])
	}
	if symbols[1].Name != "MyData" || symbols[1].Kind != SymbolStruct {
		t.Errorf("unexpected second symbol: %+v", symbols[1])
	}
}

func TestRegexExtractor_EmptyInput(t *testing.T) {
	ext := NewRegexExtractor()
	symbols, err := ext.Extract([]byte(""), "go")
	if err != nil {
		t.Fatal(err)
	}
	if len(symbols) != 0 {
		t.Errorf("expected no symbols for empty input, got %d", len(symbols))
	}
}

func TestRegexExtractor_GoMethodScope(t *testing.T) {
	source := []byte(`func (s *Server) Handle(w http.ResponseWriter, r *http.Request) {
}`)

	ext := NewRegexExtractor()
	symbols, err := ext.Extract(source, "go")
	if err != nil {
		t.Fatal(err)
	}

	var found bool
	for _, sym := range symbols {
		if sym.Name == "Handle" && sym.Kind == SymbolMethod {
			found = true
			if sym.Scope != "Server" {
				t.Errorf("expected scope 'Server', got %q", sym.Scope)
			}
		}
	}
	if !found {
		t.Error("expected Handle method")
	}
}

// --- TreeSitterExtractor Tests ---

// mockParser implements ASTParser for testing.
type mockParser struct {
	root *ASTNode
}

func (p *mockParser) Parse(source []byte) (*ASTNode, error) {
	return p.root, nil
}

func TestTreeSitterExtractor_BasicExtraction(t *testing.T) {
	// Simulate a parsed Go AST.
	root := &ASTNode{
		Type: "source_file",
		Children: []*ASTNode{
			{
				Type: "function_declaration",
				Children: []*ASTNode{
					{Type: "identifier", Content: "main", FieldName: "name"},
					{Type: "parameter_list", FieldName: "parameters"},
					{Type: "block", FieldName: "body"},
				},
			},
			{
				Type: "type_spec",
				Children: []*ASTNode{
					{Type: "type_identifier", Content: "Config", FieldName: "name"},
				},
			},
		},
	}

	ext := NewTreeSitterExtractor()
	ext.AddLanguage("go", &mockParser{root: root}, nil)

	symbols, err := ext.Extract(nil, "go")
	if err != nil {
		t.Fatal(err)
	}

	if len(symbols) < 1 {
		t.Fatal("expected at least 1 symbol")
	}

	found := make(map[string]SymbolKind)
	for _, sym := range symbols {
		found[sym.Name] = sym.Kind
	}

	if kind, ok := found["main"]; !ok || kind != SymbolFunction {
		t.Errorf("expected main function, got %v", found)
	}
	if kind, ok := found["Config"]; !ok || kind != SymbolType {
		t.Errorf("expected Config type, got %v", found)
	}
}

func TestTreeSitterExtractor_ScopeDetection(t *testing.T) {
	// Simulate a Python class with a method.
	root := &ASTNode{
		Type: "module",
		Children: []*ASTNode{
			{
				Type: "class_definition",
				Children: []*ASTNode{
					{Type: "identifier", Content: "Animal", FieldName: "name"},
					{
						Type:      "block",
						FieldName: "body",
						Children: []*ASTNode{
							{
								Type: "function_definition",
								Children: []*ASTNode{
									{Type: "identifier", Content: "speak", FieldName: "name"},
								},
							},
						},
					},
				},
			},
		},
	}

	ext := NewTreeSitterExtractor()
	ext.AddLanguage("python", &mockParser{root: root}, nil)

	symbols, err := ext.Extract(nil, "python")
	if err != nil {
		t.Fatal(err)
	}

	var speakSym *Symbol
	for i, sym := range symbols {
		if sym.Name == "speak" {
			speakSym = &symbols[i]
			break
		}
	}

	if speakSym == nil {
		t.Fatal("expected speak symbol")
	}
	if speakSym.Scope != "Animal" {
		t.Errorf("expected scope 'Animal', got %q", speakSym.Scope)
	}
}

func TestTreeSitterExtractor_UnregisteredLanguage(t *testing.T) {
	ext := NewTreeSitterExtractor()
	_, err := ext.Extract([]byte("code"), "unknown")
	if err == nil {
		t.Error("expected error for unregistered language")
	}
}

func TestTreeSitterExtractor_SupportedLanguages(t *testing.T) {
	ext := NewTreeSitterExtractor()
	ext.AddLanguage("go", &mockParser{}, nil)
	ext.AddLanguage("python", &mockParser{}, nil)

	langs := ext.SupportedLanguages()
	if len(langs) != 2 {
		t.Errorf("expected 2 languages, got %d", len(langs))
	}
}

// --- ASTNode Tests ---

func TestASTNode_ChildByFieldName(t *testing.T) {
	node := &ASTNode{
		Type: "function_declaration",
		Children: []*ASTNode{
			{Type: "identifier", Content: "foo", FieldName: "name"},
			{Type: "parameter_list", FieldName: "parameters"},
		},
	}

	name := node.ChildByFieldName("name")
	if name == nil || name.Content != "foo" {
		t.Errorf("expected child 'foo', got %v", name)
	}

	missing := node.ChildByFieldName("nonexistent")
	if missing != nil {
		t.Error("expected nil for nonexistent field")
	}
}

func TestASTNode_Walk(t *testing.T) {
	root := &ASTNode{
		Type: "root",
		Children: []*ASTNode{
			{Type: "a", Children: []*ASTNode{
				{Type: "b"},
				{Type: "c"},
			}},
			{Type: "d"},
		},
	}

	var visited []string
	root.Walk(func(n *ASTNode) bool {
		visited = append(visited, n.Type)
		return true
	})

	if len(visited) != 5 {
		t.Errorf("expected 5 nodes visited, got %d: %v", len(visited), visited)
	}
}

// --- TagExtractor Tests ---

func TestTagExtractor_Extract(t *testing.T) {
	source := []byte(`package main

func main() {
	fmt.Println("hello")
}

type Config struct {
	Name string
}
`)

	ext := NewTagExtractor()
	symbols, err := ext.Extract(source, "go")
	if err != nil {
		t.Fatal(err)
	}

	if len(symbols) == 0 {
		t.Fatal("expected symbols")
	}

	found := make(map[string]bool)
	for _, sym := range symbols {
		found[sym.Name] = true
	}
	if !found["main"] {
		t.Error("expected main function")
	}
	if !found["Config"] {
		t.Error("expected Config struct")
	}
}

func TestTagExtractor_ExtractTags(t *testing.T) {
	source := []byte(`func GetUser(id int) *User { return nil }
type User struct { Name string }
`)

	ext := NewTagExtractor()
	tags, err := ext.ExtractTags(source, "go", "user.go")
	if err != nil {
		t.Fatal(err)
	}

	if len(tags) < 2 {
		t.Fatalf("expected at least 2 tags, got %d", len(tags))
	}

	for _, tag := range tags {
		if tag.File != "user.go" {
			t.Errorf("expected file 'user.go', got %q", tag.File)
		}
		if tag.Kind == "" {
			t.Error("expected non-empty kind")
		}
	}
}

func TestFormatTags(t *testing.T) {
	tags := []Tag{
		{Name: "main", File: "main.go", Line: 3, Kind: "f"},
		{Name: "Config", File: "config.go", Line: 5, Kind: "s", Scope: "main"},
	}

	output := FormatTags(tags)

	if !strings.Contains(output, "main\tmain.go\t3;\"") {
		t.Errorf("unexpected format: %s", output)
	}
	if !strings.Contains(output, "scope:main") {
		t.Errorf("expected scope in output: %s", output)
	}
}

func TestParseTags(t *testing.T) {
	input := `!_TAG_FILE_FORMAT	2
!_TAG_FILE_SORTED	1
main	main.go	3;"	f
Config	config.go	5;"	s	scope:main	signature:type Config struct
getUserById	user.go	10;"	f
`

	symbols, err := ParseTags(input)
	if err != nil {
		t.Fatal(err)
	}

	if len(symbols) != 3 {
		t.Fatalf("expected 3 symbols, got %d", len(symbols))
	}

	if symbols[0].Name != "main" || symbols[0].Kind != SymbolFunction {
		t.Errorf("unexpected first symbol: %+v", symbols[0])
	}
	if symbols[1].Name != "Config" || symbols[1].Kind != SymbolStruct {
		t.Errorf("unexpected second symbol: %+v", symbols[1])
	}
	if symbols[1].Scope != "main" {
		t.Errorf("expected scope 'main', got %q", symbols[1].Scope)
	}
	if symbols[1].Signature != "type Config struct" {
		t.Errorf("expected signature, got %q", symbols[1].Signature)
	}
}

func TestParseTags_Empty(t *testing.T) {
	symbols, err := ParseTags("")
	if err != nil {
		t.Fatal(err)
	}
	if len(symbols) != 0 {
		t.Errorf("expected 0 symbols, got %d", len(symbols))
	}
}

// --- CompositeExtractor Tests ---

func TestCompositeExtractor(t *testing.T) {
	// Create a regex extractor.
	regex := NewRegexExtractor()

	// Create a mock tree-sitter extractor with additional symbols.
	root := &ASTNode{
		Type: "source_file",
		Children: []*ASTNode{
			{
				Type: "function_declaration",
				Children: []*ASTNode{
					{Type: "identifier", Content: "extraFunc", FieldName: "name"},
				},
			},
		},
	}
	ts := NewTreeSitterExtractor()
	ts.AddLanguage("go", &mockParser{root: root}, nil)

	composite := NewCompositeExtractor(regex, ts)

	source := []byte(`package main
func main() {}
`)
	symbols, err := composite.Extract(source, "go")
	if err != nil {
		t.Fatal(err)
	}

	found := make(map[string]bool)
	for _, sym := range symbols {
		found[sym.Name] = true
	}

	// Should have symbols from regex.
	if !found["main"] {
		t.Error("expected main from regex extractor")
	}
	// Should have symbols from tree-sitter.
	if !found["extraFunc"] {
		t.Error("expected extraFunc from tree-sitter extractor")
	}
}

func TestCompositeExtractor_Deduplication(t *testing.T) {
	regex1 := NewRegexExtractor()
	regex2 := NewRegexExtractor()

	composite := NewCompositeExtractor(regex1, regex2)

	source := []byte(`func hello() {}`)
	symbols, err := composite.Extract(source, "go")
	if err != nil {
		t.Fatal(err)
	}

	// Count occurrences of "hello".
	count := 0
	for _, sym := range symbols {
		if sym.Name == "hello" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 occurrence of 'hello', got %d", count)
	}
}

// --- SymbolKind conversion Tests ---

func TestSymbolKindToTagKind_RoundTrip(t *testing.T) {
	kinds := []SymbolKind{
		SymbolFunction, SymbolMethod, SymbolClass, SymbolStruct,
		SymbolInterface, SymbolType, SymbolVariable, SymbolConstant,
		SymbolField, SymbolImport, SymbolPackage, SymbolModule,
		SymbolProperty, SymbolEnum, SymbolEnumValue, SymbolTrait,
	}

	for _, kind := range kinds {
		tagKind := symbolKindToTagKind(kind)
		roundTripped := tagKindToSymbolKind(tagKind)
		if roundTripped != kind {
			t.Errorf("round-trip failed for %q: tag=%q -> %q", kind, tagKind, roundTripped)
		}
	}
}

// --- TypeScript Tests ---

func TestRegexExtractor_TypeScript(t *testing.T) {
	source := []byte(`export interface UserProps {
    name: string;
}

export type UserId = string;

export enum Color {
    Red,
    Green,
    Blue,
}

export class UserService {
    getUser(id: string): User {
        return {} as User;
    }
}

export function createUser(name: string): User {
    return new UserService().getUser(name);
}
`)

	ext := NewRegexExtractor()
	symbols, err := ext.Extract(source, "typescript")
	if err != nil {
		t.Fatal(err)
	}

	found := make(map[string]SymbolKind)
	for _, sym := range symbols {
		found[sym.Name] = sym.Kind
	}

	if _, ok := found["UserProps"]; !ok {
		t.Error("expected UserProps interface")
	}
	if _, ok := found["UserId"]; !ok {
		t.Error("expected UserId type")
	}
	if _, ok := found["Color"]; !ok {
		t.Error("expected Color enum")
	}
	if _, ok := found["UserService"]; !ok {
		t.Error("expected UserService class")
	}
	if _, ok := found["createUser"]; !ok {
		t.Error("expected createUser function")
	}
}

// --- Ruby Tests ---

func TestRegexExtractor_Ruby(t *testing.T) {
	source := []byte(`require 'json'

class Animal
  attr_accessor :name

  def initialize(name)
    @name = name
  end

  def speak
    "..."
  end

  def self.create(name)
    new(name)
  end
end

module Helpers
  def help
    puts "help"
  end
end
`)

	ext := NewRegexExtractor()
	symbols, err := ext.Extract(source, "ruby")
	if err != nil {
		t.Fatal(err)
	}

	found := make(map[string]SymbolKind)
	for _, sym := range symbols {
		found[sym.Name] = sym.Kind
	}

	if _, ok := found["Animal"]; !ok {
		t.Error("expected Animal class")
	}
	if _, ok := found["Helpers"]; !ok {
		t.Error("expected Helpers module")
	}
	if _, ok := found["speak"]; !ok {
		t.Error("expected speak method")
	}
}

// --- C/C++/PHP Tests ---

func TestRegexExtractor_C(t *testing.T) {
	source := []byte(`#define MAX_SIZE 1024

struct Point {
    int x;
    int y;
};

enum Color {
    RED, GREEN, BLUE
};

typedef unsigned long size_t;

static int helper(int x) {
    return x + 1;
}

int main(int argc, char *argv[]) {
    return 0;
}
`)

	ext := NewRegexExtractor()
	symbols, err := ext.Extract(source, "c")
	if err != nil {
		t.Fatal(err)
	}

	found := make(map[string]SymbolKind)
	for _, sym := range symbols {
		found[sym.Name] = sym.Kind
	}

	if _, ok := found["MAX_SIZE"]; !ok {
		t.Error("expected MAX_SIZE constant")
	}
	if _, ok := found["Point"]; !ok {
		t.Error("expected Point struct")
	}
	if _, ok := found["Color"]; !ok {
		t.Error("expected Color enum")
	}
	if _, ok := found["main"]; !ok {
		t.Error("expected main function")
	}
}

func TestRegexExtractor_CPP(t *testing.T) {
	source := []byte(`#include <iostream>

namespace utils {

class Logger {
public:
    void log(const std::string& msg);
};

template<typename T>
class Container {
    T value;
};

}

void Logger::log(const std::string& msg) {
    std::cout << msg;
}
`)

	ext := NewRegexExtractor()
	symbols, err := ext.Extract(source, "c++")
	if err != nil {
		t.Fatal(err)
	}

	found := make(map[string]SymbolKind)
	for _, sym := range symbols {
		found[sym.Name] = sym.Kind
	}

	if _, ok := found["utils"]; !ok {
		t.Error("expected utils namespace")
	}
	if _, ok := found["Logger"]; !ok {
		t.Error("expected Logger class")
	}
	if _, ok := found["Container"]; !ok {
		t.Error("expected Container template class")
	}
}

func TestRegexExtractor_CPP_MethodScope(t *testing.T) {
	source := []byte(`void MyClass::doStuff(int x) {
}`)

	ext := NewRegexExtractor()
	symbols, err := ext.Extract(source, "cpp")
	if err != nil {
		t.Fatal(err)
	}

	var found bool
	for _, sym := range symbols {
		if sym.Name == "doStuff" && sym.Kind == SymbolMethod {
			found = true
			if sym.Scope != "MyClass" {
				t.Errorf("expected scope 'MyClass', got %q", sym.Scope)
			}
		}
	}
	if !found {
		t.Error("expected doStuff method with scope")
	}
}

func TestRegexExtractor_PHP(t *testing.T) {
	source := []byte(`<?php
namespace App\Controllers;

use App\Models\User;

abstract class BaseController {
    public function index() {
        return [];
    }
}

interface Renderable {
    public function render();
}

trait Cacheable {
    public function cache() {}
}

function helper() {
    return true;
}

const VERSION = "1.0";
`)

	ext := NewRegexExtractor()
	symbols, err := ext.Extract(source, "php")
	if err != nil {
		t.Fatal(err)
	}

	found := make(map[string]SymbolKind)
	for _, sym := range symbols {
		found[sym.Name] = sym.Kind
	}

	if _, ok := found["BaseController"]; !ok {
		t.Error("expected BaseController class")
	}
	if _, ok := found["Renderable"]; !ok {
		t.Error("expected Renderable interface")
	}
	if _, ok := found["Cacheable"]; !ok {
		t.Error("expected Cacheable trait")
	}
	if _, ok := found["helper"]; !ok {
		t.Error("expected helper function")
	}
}

// --- TreeSitter Custom Rules Test ---

func TestTreeSitterExtractor_CustomRules(t *testing.T) {
	root := &ASTNode{
		Type: "source",
		Children: []*ASTNode{
			{
				Type: "custom_func",
				Children: []*ASTNode{
					{Type: "identifier", Content: "myFunc", FieldName: "fname"},
				},
			},
			{
				Type: "custom_type",
				Children: []*ASTNode{
					{Type: "identifier", Content: "MyType", FieldName: "tname"},
				},
			},
		},
	}

	customRules := []ExtractionRule{
		{NodeType: "custom_func", Kind: SymbolFunction, NameField: "fname"},
		{NodeType: "custom_type", Kind: SymbolType, NameField: "tname"},
	}

	ext := NewTreeSitterExtractor()
	ext.AddLanguage("custom", &mockParser{root: root}, customRules)

	symbols, err := ext.Extract(nil, "custom")
	if err != nil {
		t.Fatal(err)
	}

	if len(symbols) != 2 {
		t.Fatalf("expected 2 symbols, got %d", len(symbols))
	}

	found := make(map[string]SymbolKind)
	for _, sym := range symbols {
		found[sym.Name] = sym.Kind
	}
	if found["myFunc"] != SymbolFunction {
		t.Error("expected myFunc function")
	}
	if found["MyType"] != SymbolType {
		t.Error("expected MyType type")
	}
}

func TestTreeSitterExtractor_GenericRulesFallback(t *testing.T) {
	// When no rules are registered for a language, generic rules are used.
	root := &ASTNode{
		Type: "source",
		Children: []*ASTNode{
			{
				Type: "function_declaration",
				Children: []*ASTNode{
					{Type: "identifier", Content: "genericFunc", FieldName: "name"},
				},
			},
		},
	}

	ext := NewTreeSitterExtractor()
	// Register parser but remove rules for this language.
	ext.AddLanguage("nolang", &mockParser{root: root}, nil)
	delete(ext.rules, "nolang") // force generic fallback

	symbols, err := ext.Extract(nil, "nolang")
	if err != nil {
		t.Fatal(err)
	}

	if len(symbols) != 1 || symbols[0].Name != "genericFunc" {
		t.Errorf("expected genericFunc via generic rules, got %v", symbols)
	}
}

func TestTreeSitterExtractor_LanguageAlias(t *testing.T) {
	root := &ASTNode{Type: "source"}
	ext := NewTreeSitterExtractor()
	ext.AddLanguage("go", &mockParser{root: root}, nil)

	// "golang" should resolve to "go".
	_, err := ext.Extract(nil, "golang")
	if err != nil {
		t.Errorf("expected alias 'golang' to resolve to 'go', got error: %v", err)
	}
}

// --- ASTNode Additional Tests ---

func TestASTNode_ChildrenByType(t *testing.T) {
	node := &ASTNode{
		Type: "parent",
		Children: []*ASTNode{
			{Type: "identifier", Content: "a"},
			{Type: "identifier", Content: "b"},
			{Type: "block", Content: "{}"},
			{Type: "identifier", Content: "c"},
		},
	}

	idents := node.ChildrenByType("identifier")
	if len(idents) != 3 {
		t.Errorf("expected 3 identifiers, got %d", len(idents))
	}

	blocks := node.ChildrenByType("block")
	if len(blocks) != 1 {
		t.Errorf("expected 1 block, got %d", len(blocks))
	}

	missing := node.ChildrenByType("nonexistent")
	if len(missing) != 0 {
		t.Errorf("expected 0 for nonexistent type, got %d", len(missing))
	}
}

func TestASTNode_WalkEarlyStop(t *testing.T) {
	root := &ASTNode{
		Type: "root",
		Children: []*ASTNode{
			{Type: "a", Children: []*ASTNode{{Type: "a_child"}}},
			{Type: "b", Children: []*ASTNode{{Type: "c"}}},
		},
	}

	var visited []string
	root.Walk(func(n *ASTNode) bool {
		visited = append(visited, n.Type)
		return n.Type != "a" // returning false for "a" skips its children
	})

	// Walk visits root, then "a" (returns false so "a_child" is skipped),
	// then continues to sibling "b" and its child "c".
	expected := []string{"root", "a", "b", "c"}
	if len(visited) != len(expected) {
		t.Errorf("expected %d nodes visited, got %d: %v", len(expected), len(visited), visited)
	}
	for i, e := range expected {
		if i < len(visited) && visited[i] != e {
			t.Errorf("position %d: expected %q, got %q", i, e, visited[i])
		}
	}
}

// --- Tag Round-Trip Test ---

func TestTagExtractor_RoundTrip(t *testing.T) {
	source := []byte(`package main

func main() {}
type Config struct {}
const Version = "1.0"
`)

	// Extract tags.
	ext := NewTagExtractor()
	tags, err := ext.ExtractTags(source, "go", "main.go")
	if err != nil {
		t.Fatal(err)
	}

	// Format to ctags string.
	formatted := FormatTags(tags)

	// Parse back.
	symbols, err := ParseTags(formatted)
	if err != nil {
		t.Fatal(err)
	}

	// Original and parsed should have matching names.
	origNames := make(map[string]bool)
	for _, tag := range tags {
		origNames[tag.Name] = true
	}
	for _, sym := range symbols {
		if !origNames[sym.Name] {
			t.Errorf("round-trip lost symbol %q", sym.Name)
		}
	}
	if len(symbols) != len(tags) {
		t.Errorf("round-trip count mismatch: %d tags -> %d symbols", len(tags), len(symbols))
	}
}

func TestParseTags_MalformedLines(t *testing.T) {
	// Should silently skip malformed lines.
	input := "justOnefield\n\ntoo\tfew\nvalid\tfile.go\t3;\"\tf\n"
	symbols, err := ParseTags(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(symbols) != 1 {
		t.Errorf("expected 1 valid symbol, got %d", len(symbols))
	}
	if symbols[0].Name != "valid" {
		t.Errorf("expected 'valid', got %q", symbols[0].Name)
	}
}

// --- Specificity Tests ---

func TestKindSpecificity(t *testing.T) {
	// Struct should beat type.
	if kindSpecificity(SymbolStruct) <= kindSpecificity(SymbolType) {
		t.Error("struct should outrank type")
	}
	// Function should beat variable.
	if kindSpecificity(SymbolFunction) <= kindSpecificity(SymbolVariable) {
		t.Error("function should outrank variable")
	}
	// Package (default) should be lowest.
	if kindSpecificity(SymbolPackage) != 0 {
		t.Error("package should have specificity 0")
	}
}

// --- SupportedLanguages Tests ---

func TestRegexExtractor_SupportedLanguages(t *testing.T) {
	ext := NewRegexExtractor()
	langs := ext.SupportedLanguages()

	expected := []string{"go", "python", "javascript", "typescript", "java", "rust", "c", "c++", "ruby", "php"}
	for _, lang := range expected {
		found := false
		for _, l := range langs {
			if l == lang {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected language %q in supported list", lang)
		}
	}
}
