package ast

import (
	"fmt"
	"go/token"
	"log"
	"regexp"
	"strings"

	"github.com/ListenOcean/goHookTool/configs"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

// The hookpoint structure holds every AST node required during the
// instrumentation of a file.
type Hookpoint struct {
	DescriptorFuncDecl  *dst.FuncDecl
	PrologVarDecl       *dst.GenDecl
	PrologLoadFuncDecl  *dst.FuncDecl
	InstrumentationStmt dst.Stmt
}

func GetHookpoint(signatrue, id string, funcDecl *dst.FuncDecl, descriptorValueInitializer HookDescriptorValueInitializer) *Hookpoint {
	epilogFuncType, epilogCallArgs := newEpilogFuncType(funcDecl.Type)
	prologFuncType, prologCallArgs := newPrologFuncType(funcDecl, epilogFuncType)

	prologVarIdent := fmt.Sprintf(configs.PrologVarIdentFormat, id)
	prologVarDecl, prologValueSpec := newPrologVarDecl(prologVarIdent, prologFuncType)

	prologLoadFuncIdent := fmt.Sprintf(configs.PrologLoadFuncIdentFormat, id)
	prologLoadFuncDecl := newPrologLoadFuncDecl(prologLoadFuncIdent, prologValueSpec)

	descriptorFuncIdent := fmt.Sprintf(configs.HookDescriptorFuncIdentFormat, id)
	descriptorFuncDecl := newHookDescriptorFuncDecl(descriptorFuncIdent, funcDecl, prologVarIdent, descriptorValueInitializer)

	instrumentationStmt := newInstrumentationStmt(prologLoadFuncIdent, prologCallArgs, epilogCallArgs, id, signatrue)

	return &Hookpoint{
		PrologLoadFuncDecl:  prologLoadFuncDecl,
		DescriptorFuncDecl:  descriptorFuncDecl,
		PrologVarDecl:       prologVarDecl,
		InstrumentationStmt: instrumentationStmt,
	}
}

func NewHookpoint(signatrue string, pkgPath string, funcDecl *dst.FuncDecl, descriptorTypeIdent string, descriptorValueInitializer HookDescriptorValueInitializer) *Hookpoint {
	id := normalizedHookpointID(pkgPath, funcDecl)
	log.Printf("Hookpoint id: %s\n", id)
	return GetHookpoint(signatrue, id, funcDecl, descriptorValueInitializer)
}

func normalizedHookpointID(pkgPath string, node *dst.FuncDecl) string {
	var receiver string
	if node.Recv != nil {
		t := node.Recv.List[0].Type
	loop:
		for {
			switch actual := t.(type) {
			default:
				log.Fatalf("unexpected type %T\n", actual)

			case *dst.StarExpr:
				t = actual.X

			case *dst.Ident:
				receiver = actual.Name
				break loop
			}
		}
		receiver += "_"
	}
	pkgPath = normalizedPkgPath(pkgPath)
	return fmt.Sprintf("%s_%s%s", pkgPath, receiver, node.Name)
}

func normalizedPkgPath(pkgPath string) string {
	return regexp.MustCompile(`[/.\-@]`).ReplaceAllString(pkgPath, "_")
}

// Return the global prolog variable declaration.
func newPrologVarDecl(ident string, typ dst.Expr) (*dst.GenDecl, *dst.ValueSpec) {
	typ = &dst.StarExpr{X: dst.Clone(typ).(dst.Expr)}
	return newVarDecl(ident, typ)
}

// Return the function declaration loading the global prolog variable using
func newPrologLoadFuncDecl(ident string, prologVarSpec *dst.ValueSpec) *dst.FuncDecl {
	prologVarName := prologVarSpec.Names[0].Name
	prologVarType := prologVarSpec.Type
	retType := dst.Clone(prologVarType).(dst.Expr)
	retCastType := dst.Clone(prologVarType).(dst.Expr)

	return &dst.FuncDecl{
		Name: dst.NewIdent(ident),
		Type: &dst.FuncType{
			Params: &dst.FieldList{},
			Results: &dst.FieldList{
				List: []*dst.Field{
					{
						Type: retType,
					},
				},
			},
		},
		Body: &dst.BlockStmt{
			List: []dst.Stmt{
				&dst.ReturnStmt{
					Results: []dst.Expr{
						&dst.CallExpr{
							Fun: retCastType,
							Args: []dst.Expr{
								&dst.CallExpr{
									Fun: dst.NewIdent(configs.AtomicLoadPointerFuncIdent),
									Args: []dst.Expr{
										newCastValueExpr(
											newUnsafePointerType(),
											newIdentAddressExpr(dst.NewIdent(prologVarName))),
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func GetEpilogBody(epilogCallArgs []dst.Expr, signatrue string) *dst.BlockStmt {
	if customCode, ok := configs.ConfigData.Codes[signatrue]; ok {
		if customCode.Epilog != "" {
			return &dst.BlockStmt{
				List: GetBlockAst(customCode.Epilog),
			}
		}
	}

	epilogDefer := dst.DeferStmt{
		Call: &dst.CallExpr{
			Fun: &dst.FuncLit{
				Type: &dst.FuncType{
					Func:   true,
					Params: &dst.FieldList{},
				},
				Body: &dst.BlockStmt{
					List: []dst.Stmt{
						&dst.ExprStmt{
							X: &dst.CallExpr{
								Fun: dst.NewIdent(configs.EpilogVarIdent),
								// 尾言参数
								Args: epilogCallArgs,
							},
						},
					},
				},
			},
		},
	}

	return &dst.BlockStmt{
		List: []dst.Stmt{
			&epilogDefer,
		},
	}
}

func GetProloglogBody(prologCallArgs []dst.Expr, signatrue string) []dst.Stmt {
	if customCode, ok := configs.ConfigData.Codes[signatrue]; ok {
		if customCode.Prolog != "" {
			return GetBlockAst(customCode.Prolog)
		}
	}
	return []dst.Stmt{
		&dst.AssignStmt{
			Lhs: []dst.Expr{
				dst.NewIdent(configs.EpilogVarIdent),
				dst.NewIdent(configs.PrologAbortErrorVarIdent),
			},
			Tok: token.DEFINE,
			Rhs: []dst.Expr{
				&dst.CallExpr{
					Fun:      &dst.StarExpr{X: dst.NewIdent(configs.PrologVarIdent)},
					Args:     prologCallArgs,
					Ellipsis: false,
					Decs:     dst.CallExprDecorations{},
				},
			},
		},
	}
}

// Return the instrumentation statement node to be added to a function body.
func newInstrumentationStmt(prologLoadFuncIdent string, prologCallArgs, epilogCallArgs []dst.Expr, id string, signatrue string) dst.Stmt {

	epilogBody := GetEpilogBody(epilogCallArgs, signatrue)
	prologBody := GetProloglogBody(prologCallArgs, signatrue)

	return &dst.BlockStmt{
		List: []dst.Stmt{
			// if _prolog := <prologLoadFuncIdent>(); _prolog != nil { ... }
			&dst.IfStmt{
				Init: &dst.AssignStmt{
					Lhs: []dst.Expr{dst.NewIdent(configs.PrologVarIdent)},
					Tok: token.DEFINE,
					Rhs: []dst.Expr{&dst.CallExpr{Args: []dst.Expr{}, Fun: dst.NewIdent(prologLoadFuncIdent)}},
				},
				Cond: &dst.BinaryExpr{
					X:  dst.NewIdent(configs.PrologVarIdent),
					Op: token.NEQ,
					Y:  dst.NewIdent(configs.NilIdent),
				},
				Body: &dst.BlockStmt{
					List: append(
						// default is: _epilog, _prolog_abort_err := (*_prolog)(<args>)
						prologBody,
						// default is: if _epilog != nil { defer _epilog(<args>) }
						&dst.IfStmt{
							Cond: &dst.BinaryExpr{
								X:  dst.NewIdent(configs.EpilogVarIdent),
								Op: token.NEQ,
								Y:  dst.NewIdent(configs.NilIdent),
							},
							Body: epilogBody,
						},
						// default is: if _prolog_abort_err != nil { return }
						&dst.IfStmt{
							Cond: &dst.BinaryExpr{
								X:  dst.NewIdent(configs.PrologAbortErrorVarIdent),
								Op: token.NEQ,
								Y:  dst.NewIdent(configs.NilIdent),
							},
							Body: &dst.BlockStmt{
								List: []dst.Stmt{
									&dst.ReturnStmt{},
								},
							},
						},
					),
				},
			},
		},
	}
}

// Return the epilog type of the given function type.
// `f(<params>) <results>` returns `func(<*params>) (<epilog type>, error)`
func newPrologFuncType(funcDecl *dst.FuncDecl, epilogType *dst.FuncType) (prologType *dst.FuncType, callParams []dst.Expr) {
	funcType := funcDecl.Type

	var callbackTypeParamList *dst.FieldList
	var callbackCallParams []dst.Expr
	callbackTypeParamList, callbackCallParams = newCallbackParams(funcDecl.Recv, funcType.Params, "_param")
	return &dst.FuncType{
		Params: callbackTypeParamList,
		Results: &dst.FieldList{
			List: []*dst.Field{
				{
					Type: epilogType,
				},
				{
					Type: dst.NewIdent("error"),
				},
			},
		},
	}, callbackCallParams
}

// Return the epilog type of the given function type.
// `f(<params>) <results>` returns `func(<*results>)`
func newEpilogFuncType(funcType *dst.FuncType) (epilogType *dst.FuncType, callParams []dst.Expr) {
	callbackTypeParamList, callbackCallParams := newCallbackParams(nil, funcType.Results, "_result")
	return &dst.FuncType{
		Params:  callbackTypeParamList,
		Results: &dst.FieldList{},
	}, callbackCallParams
}

// newCallbackParams walks the given function parameters and returns the
// parameter for the callback (prolog or epilog), along with the list of call
// arguments.
func newCallbackParams(recv *dst.FieldList, params *dst.FieldList, ignoredParamPrefix string) (callbackTypeParamList *dst.FieldList, callbackCallParams []dst.Expr) {
	var callbackTypeParams []*dst.Field
	var hookedParams []*dst.Field
	if recv != nil {
		hookedParams = recv.List
	}
	if params != nil {
		hookedParams = append(hookedParams, params.List...)
	}
	p := 0
	for _, hookedParam := range hookedParams {
		var callbackTypeParam *dst.Field
		callbackTypeParam = &dst.Field{Type: newCallbackParamType(hookedParam.Type)}
		if len(hookedParam.Names) == 0 {
			// Case where the parameter has no name such as f(string): no longer
			// ignore it and name it.
			// - The hooked function parameter must be named.
			hookedParam.Names = []*dst.Ident{newParamIdent(ignoredParamPrefix, p)}
			// - The callback type expects this parameter type.
			callbackTypeParams = append(callbackTypeParams, callbackTypeParam)
			// - The callback call must pass the hooked function parameter.
			callbackCallParams = append(callbackCallParams, newCallbackCallParam(newParamIdent(ignoredParamPrefix, p)))
			p++
		} else {
			// Case where the parameters are named, but still possibly ignored.
			for _, name := range hookedParam.Names {
				if name.Name == "_" {
					// Case where the parameter is ignored using `_` such as
					// f(_ string):  no longer ignore it and name it.
					*name = *newParamIdent(ignoredParamPrefix, p)
				}
				callbackTypeParam = dst.Clone(callbackTypeParam).(*dst.Field)

				// The callback type expects this parameter type.
				callbackTypeParams = append(callbackTypeParams, callbackTypeParam)
				// The callback call must pass the hooked function parameter.
				callbackCallParams = append(callbackCallParams, newCallbackCallParam(dst.NewIdent(name.Name)))
				p++
			}
		}
	}
	return &dst.FieldList{List: callbackTypeParams}, callbackCallParams
}

func newCallbackParamType(hookedParamType dst.Expr) dst.Expr {
	typ := dst.Clone(hookedParamType).(dst.Expr)
	if variadic, ok := typ.(*dst.Ellipsis); ok {
		typ = &dst.ArrayType{Elt: variadic.Elt}
	}
	// return &dst.StarExpr{X: typ}
	return typ
}

func newCallbackParamTypeWithAddress(hookedParamType dst.Expr) dst.Expr {
	typ := dst.Clone(hookedParamType).(dst.Expr)
	if variadic, ok := typ.(*dst.Ellipsis); ok {
		typ = &dst.ArrayType{Elt: variadic.Elt}
	}
	return &dst.StarExpr{X: typ}
}

func newCallbackCallParam(ident *dst.Ident) dst.Expr {
	// return newIdentAddressExpr(ident)
	return ident
}

func newCallbackCallParamWithAddress(ident *dst.Ident) dst.Expr {
	return newIdentAddressExpr(ident)
}

func newParamIdent(prefix string, p int) *dst.Ident {
	return dst.NewIdent(fmt.Sprintf("%s%d", prefix, p))
}

// Return the hook descriptor function declaration which returns the hook
// descriptor structure.
func newHookDescriptorFuncDecl(ident string, funcDecl *dst.FuncDecl, prologVarIdent string, newDescriptorValueInitializer HookDescriptorValueInitializer) *dst.FuncDecl {
	return &dst.FuncDecl{
		Decs: dst.FuncDeclDecorations{
			NodeDecs: dst.NodeDecs{
				Before: dst.NewLine,
				Start: dst.Decorations{
					"//go:nosplit", // save some instructions
					fmt.Sprintf("//go:linkname %[1]s %[1]s\n", ident),
				},
			},
		},
		Name: dst.NewIdent(ident),
		Type: &dst.FuncType{
			Params: &dst.FieldList{
				List: []*dst.Field{
					{
						Names: []*dst.Ident{dst.NewIdent(configs.HookDescriptorParamName)},
						Type:  &dst.StarExpr{X: dst.NewIdent(configs.HookDescriptorTypeIdent)},
					},
				},
			},
			Results: &dst.FieldList{},
		},
		Body: &dst.BlockStmt{
			List: []dst.Stmt{
				&dst.AssignStmt{
					Lhs: []dst.Expr{
						&dst.StarExpr{X: dst.NewIdent(configs.HookDescriptorParamName)},
					},
					Tok: token.ASSIGN,
					Rhs: []dst.Expr{
						newDescriptorValueInitializer(newFunctionValueExpr(funcDecl), newIdentAddressExpr(dst.NewIdent(prologVarIdent))),
					},
				},
			},
		},
	}
}

// Return link time function declaration for the atomic load pointer function.
func NewLinkTimeAtomicLoadPointerFuncDecl() *dst.FuncDecl {
	ftype := &dst.FuncType{
		Params: &dst.FieldList{
			List: []*dst.Field{{Type: newUnsafePointerType()}},
		},
		Results: &dst.FieldList{
			List: []*dst.Field{{Type: newUnsafePointerType()}},
		},
	}
	return newLinkTimeForwardFuncDecl(configs.AtomicLoadPointerFuncIdent, ftype)
}

type HookDescriptorValueInitializer func(Func, Prolog dst.Expr) dst.Expr

// Return the type declaration for
// ```
//
//	type _hook_descriptor_type = struct {
//	  Func, Prolog interface{}
//	}
//
// ```
func NewHookDescriptorType() (*dst.GenDecl, *dst.TypeSpec, HookDescriptorValueInitializer) {
	spec := &dst.TypeSpec{
		Name: dst.NewIdent(configs.HookDescriptorTypeIdent),
		//Assign: true,
		Type: &dst.StructType{
			Fields: &dst.FieldList{
				List: []*dst.Field{
					{
						Names: []*dst.Ident{
							dst.NewIdent("Func"),
							dst.NewIdent("Prolog"),
						},
						Type: newEmptyInterfaceType(),
					},
				},
			},
		},
	}

	typ := &dst.GenDecl{
		Tok: token.TYPE,
		Specs: []dst.Spec{
			spec,
		},
	}

	valInitializer := func(Func, Prolog dst.Expr) dst.Expr {
		return &dst.CompositeLit{
			Type: dst.NewIdent(configs.HookDescriptorTypeIdent),
			Elts: []dst.Expr{
				&dst.KeyValueExpr{
					Key:   dst.NewIdent("Func"),
					Value: Func,
				},
				&dst.KeyValueExpr{
					Key:   dst.NewIdent("Prolog"),
					Value: Prolog,
				},
			},
		}
	}

	return typ, spec, valInitializer
}

func ShouldIgnoreFuncDecl(funcDecl *dst.FuncDecl) bool {
	fname := funcDecl.Name.Name
	// don't instrument:
	// - `_`: explicitly ignored function names.
	// - `init`: package init functions.
	// - `.*noescape.*`: any function name containing `noescape` since we would
	//    likely break it.
	// - functions having //go:nosplit directives because they are usually low-level
	//   functions.
	// - functions having //autobuild:ignore directives.
	return funcDecl.Body == nil ||
		fname == "_" ||
		fname == "init" ||
		strings.Contains(fname, "noescape") ||
		HasIgnoreDirective(funcDecl) ||
		hasGoNoSplitDirective(funcDecl)
}

func IsHookDescriptorFuncInMainPackage(ident string) bool {
	return strings.HasPrefix(ident, configs.HookDescriptorFuncIdentPrefixOfMainPackage)
}

func GetBlockAst(data string) []dst.Stmt {
	file, err := decorator.Parse(fmt.Sprintf(configs.CodeTemplate, data))
	if err != nil {
		log.Printf("decorator.Parse failed, parse `%s`, err %s", data, err.Error())
	}
	return file.Decls[0].(*dst.FuncDecl).Body.List
}
