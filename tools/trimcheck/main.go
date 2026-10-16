// trimcheck 报告（-w 时删除）参数已被证明裁剪过的 strings.TrimSpace 调用。
//
// 证明规则：常量值无首尾空白；strings.TrimSpace 及所有 return 都返回已裁剪值的本模块函数
// （最大不动点）；所有赋值都已裁剪的局部变量；本模块声明、无 struct tag、不取地址、
// 不经接口参数反射写入、所有写入都已裁剪的字段。外部模块字段一律视为未裁剪。
// 由于只看到当前 GOOS 的构建文件，门禁应在 linux/darwin/windows 下各运行一次。
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"os"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

type funcInfo struct {
	decl    *ast.FuncDecl
	pkg     *packages.Package
	results []bool // per result index: always trimmed
}

type fieldWrite struct {
	p    *packages.Package
	fd   *ast.FuncDecl
	expr ast.Expr // nil means non-trimmed write
}

var (
	fieldWrites = map[string][]fieldWrite{}
	fieldBad    = map[string]bool{}
	fieldState  = map[string]int{}
	fset        *token.FileSet
	scopes      = map[*ast.FuncDecl]*scope{}
	funcs       = map[string]*funcInfo{}
	byDecl      = map[*ast.FuncDecl]*funcInfo{}
	trimFn      *types.Func
	apply       = flag.Bool("w", false, "rewrite files")
	only        = flag.String("only", "", "file of positions allowed to rewrite")
)

func isTrimSpace(p *packages.Package, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	obj, ok := p.TypesInfo.Uses[sel.Sel].(*types.Func)
	return ok && obj.FullName() == "strings.TrimSpace"
}

func calleeFunc(p *packages.Package, call *ast.CallExpr) *types.Func {
	var id *ast.Ident
	switch f := call.Fun.(type) {
	case *ast.Ident:
		id = f
	case *ast.SelectorExpr:
		id = f.Sel
	default:
		return nil
	}
	fn, _ := p.TypesInfo.Uses[id].(*types.Func)
	if fn == nil {
		return nil
	}
	if sig, ok := fn.Type().(*types.Signature); ok && sig.Recv() != nil {
		if _, isIface := sig.Recv().Type().Underlying().(*types.Interface); isIface {
			return nil
		}
	}
	return fn.Origin()
}

// localTrimmed caches per-function variable analysis.
type scope struct {
	p       *packages.Package
	body    *ast.BlockStmt
	vars    map[*types.Var]int // 0 unknown,1 trimmed,2 not
	assigns map[*types.Var][]func() bool
	bad     map[*types.Var]bool
}

func newScope(p *packages.Package, fd *ast.FuncDecl) *scope {
	s := &scope{p: p, body: fd.Body, vars: map[*types.Var]int{}, assigns: map[*types.Var][]func() bool{}, bad: map[*types.Var]bool{}}
	// parameters and named results are not trimmed
	mark := func(fl *ast.FieldList) {
		if fl == nil {
			return
		}
		for _, f := range fl.List {
			for _, n := range f.Names {
				if v, ok := p.TypesInfo.Defs[n].(*types.Var); ok {
					s.bad[v] = true
				}
			}
		}
	}
	mark(fd.Type.Params)
	mark(fd.Type.Results)
	if fd.Recv != nil {
		mark(fd.Recv)
	}
	varOf := func(e ast.Expr) *types.Var {
		id, ok := e.(*ast.Ident)
		if !ok {
			return nil
		}
		if v, ok := p.TypesInfo.Defs[id].(*types.Var); ok {
			return v
		}
		v, _ := p.TypesInfo.Uses[id].(*types.Var)
		return v
	}
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncLit:
			mark(x.Type.Params)
			mark(x.Type.Results)
		case *ast.UnaryExpr:
			if x.Op == token.AND {
				if v := varOf(x.X); v != nil {
					s.bad[v] = true
				}
			}
		case *ast.IncDecStmt:
			if v := varOf(x.X); v != nil {
				s.bad[v] = true
			}
		case *ast.RangeStmt:
			for _, e := range []ast.Expr{x.Key, x.Value} {
				if e != nil {
					if v := varOf(e); v != nil {
						s.bad[v] = true
					}
				}
			}
		case *ast.TypeSwitchStmt:
			// bound vars in type switches: mark all implicit objects bad
			for _, c := range x.Body.List {
				if obj, ok := p.TypesInfo.Implicits[c].(*types.Var); ok {
					s.bad[obj] = true
				}
			}
		case *ast.ValueSpec:
			for i, n := range x.Names {
				v, _ := p.TypesInfo.Defs[n].(*types.Var)
				if v == nil {
					continue
				}
				if len(x.Values) == 0 {
					s.assigns[v] = append(s.assigns[v], func() bool { return true })
				} else if len(x.Values) == len(x.Names) {
					e := x.Values[i]
					s.assigns[v] = append(s.assigns[v], func() bool { return s.trimmed(e) })
				} else {
					call, _ := x.Values[0].(*ast.CallExpr)
					idx := i
					s.assigns[v] = append(s.assigns[v], func() bool { return call != nil && s.callResultTrimmed(call, idx) })
				}
			}
		case *ast.AssignStmt:
			for i, lhs := range x.Lhs {
				v := varOf(lhs)
				if v == nil {
					continue
				}
				if x.Tok != token.ASSIGN && x.Tok != token.DEFINE {
					s.bad[v] = true
					continue
				}
				if len(x.Rhs) == len(x.Lhs) {
					e := x.Rhs[i]
					s.assigns[v] = append(s.assigns[v], func() bool { return s.trimmed(e) })
				} else {
					call, _ := x.Rhs[0].(*ast.CallExpr)
					idx := i
					s.assigns[v] = append(s.assigns[v], func() bool { return call != nil && s.callResultTrimmed(call, idx) })
				}
			}
		}
		return true
	})
	return s
}

func (s *scope) varTrimmed(v *types.Var) bool {
	if s.bad[v] || v.Parent() == nil || v.Parent() == v.Pkg().Scope() {
		return false
	}
	switch s.vars[v] {
	case 1:
		return true
	case 2, 3:
		return false
	}
	as := s.assigns[v]
	if len(as) == 0 {
		s.vars[v] = 2
		return false
	}
	s.vars[v] = 3 // in progress: assume not (conservative for cycles)
	for _, a := range as {
		if !a() {
			s.vars[v] = 2
			return false
		}
	}
	s.vars[v] = 1
	return true
}

func (s *scope) callResultTrimmed(call *ast.CallExpr, idx int) bool {
	if idx == 0 && isTrimSpace(s.p, call) {
		return true
	}
	fn := calleeFunc(s.p, call)
	if fn == nil {
		return false
	}
	info := funcs[objKey(fn)]
	return info != nil && idx < len(info.results) && info.results[idx]
}

func (s *scope) trimmed(e ast.Expr) bool {
	e = ast.Unparen(e)
	if tv, ok := s.p.TypesInfo.Types[e]; ok && tv.Value != nil && tv.Value.Kind() == constant.String {
		v := constant.StringVal(tv.Value)
		return v == strings.TrimSpace(v)
	}
	switch x := e.(type) {
	case *ast.CallExpr:
		return s.callResultTrimmed(x, 0)
	case *ast.Ident:
		if v, ok := s.p.TypesInfo.Uses[x].(*types.Var); ok {
			if v.IsField() {
				return fieldTrimmed(v)
			}
			return s.varTrimmed(v)
		}
	case *ast.SelectorExpr:
		if v, ok := s.p.TypesInfo.Uses[x.Sel].(*types.Var); ok && v.IsField() {
			return fieldTrimmed(v)
		}
	}
	return false
}

func objKey(o types.Object) string { return fset.Position(o.Pos()).String() + "#" + o.Name() }

func fieldTrimmed(fv *types.Var) bool {
	// 只有本模块声明的字段才能看到全部写入；外部模块（SDK 等）的字段可能在别处被写入。
	if fv.Pkg() == nil || !strings.HasPrefix(fv.Pkg().Path(), "github.com/nexus-research-lab/nexus/") {
		return false
	}
	v := objKey(fv.Origin())
	if fieldBad[v] {
		return false
	}
	switch fieldState[v] {
	case 1:
		return true
	case 2, 3:
		return false
	}
	ws := fieldWrites[v]
	if len(ws) == 0 {
		fieldState[v] = 2
		return false
	}
	fieldState[v] = 3
	for _, w := range ws {
		if w.expr == nil || w.fd == nil {
			fieldState[v] = 2
			return false
		}
		sc := scopes[w.fd]
		if sc == nil {
			sc = newScope(w.p, w.fd)
			scopes[w.fd] = sc
		}
		if !sc.trimmed(w.expr) {
			fieldState[v] = 2
			return false
		}
	}
	fieldState[v] = 1
	return true
}

func structOf(t types.Type) *types.Struct {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	st, _ := t.Underlying().(*types.Struct)
	return st
}

func markStructBad(st *types.Struct, exportedOnly bool) {
	if st == nil {
		return
	}
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		if !exportedOnly || f.Exported() {
			fieldBad[objKey(f.Origin())] = true
		}
		if f.Embedded() {
			markStructBad(structOf(f.Type()), exportedOnly)
		}
	}
}

func collectFieldWrites(p *packages.Package) {
	for _, f := range p.Syntax {
		var cur *ast.FuncDecl
		var visit func(n ast.Node) bool
		fieldOf := func(e ast.Expr) *types.Var {
			sel, ok := ast.Unparen(e).(*ast.SelectorExpr)
			if !ok {
				return nil
			}
			v, ok := p.TypesInfo.Uses[sel.Sel].(*types.Var)
			if ok && v.IsField() {
				return v.Origin()
			}
			return nil
		}
		visit = func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.StructType:
				for _, fl := range x.Fields.List {
					if fl.Tag != nil {
						for _, nm := range fl.Names {
							if v, ok := p.TypesInfo.Defs[nm].(*types.Var); ok {
								fieldBad[objKey(v.Origin())] = true
							}
						}
					}
				}
			case *ast.AssignStmt:
				for i, lhs := range x.Lhs {
					fv := fieldOf(lhs)
					if fv == nil {
						continue
					}
					if x.Tok != token.ASSIGN || len(x.Rhs) != len(x.Lhs) {
						fieldBad[objKey(fv)] = true
						continue
					}
					fieldWrites[objKey(fv)] = append(fieldWrites[objKey(fv)], fieldWrite{p, cur, x.Rhs[i]})
				}
			case *ast.IncDecStmt:
				if fv := fieldOf(x.X); fv != nil {
					fieldBad[objKey(fv)] = true
				}
			case *ast.RangeStmt:
				for _, e := range []ast.Expr{x.Key, x.Value} {
					if fv := fieldOf(e); fv != nil {
						fieldBad[objKey(fv)] = true
					}
				}
			case *ast.UnaryExpr:
				if x.Op == token.AND {
					if fv := fieldOf(x.X); fv != nil {
						fieldBad[objKey(fv)] = true
					}
				}
			case *ast.CompositeLit:
				tv, ok := p.TypesInfo.Types[x]
				if !ok {
					return true
				}
				st := structOf(tv.Type)
				if st == nil {
					return true
				}
				for i, el := range x.Elts {
					if kv, ok := el.(*ast.KeyValueExpr); ok {
						id, _ := kv.Key.(*ast.Ident)
						if id == nil {
							continue
						}
						if v, ok := p.TypesInfo.Uses[id].(*types.Var); ok && v.IsField() {
							fieldWrites[objKey(v.Origin())] = append(fieldWrites[objKey(v.Origin())], fieldWrite{p, cur, kv.Value})
						}
					} else if i < st.NumFields() {
						k := objKey(st.Field(i).Origin())
						fieldWrites[k] = append(fieldWrites[k], fieldWrite{p, cur, el})
					}
				}
			case *ast.CallExpr:
				// conversions into struct types copy foreign field values
				if tv, ok := p.TypesInfo.Types[x.Fun]; ok && tv.IsType() {
					markStructBad(structOf(tv.Type), false)
					return true
				}
				// pointers to structs passed as interface args may be filled by reflection
				sig, _ := p.TypesInfo.TypeOf(x.Fun).(*types.Signature)
				for i, a := range x.Args {
					at := p.TypesInfo.TypeOf(a)
					if at == nil {
						continue
					}
					var pt types.Type
					if sig != nil {
						if sig.Variadic() && i >= sig.Params().Len()-1 {
							last := sig.Params().At(sig.Params().Len() - 1).Type()
							if sl, ok := last.(*types.Slice); ok {
								pt = sl.Elem()
							}
						} else if i < sig.Params().Len() {
							pt = sig.Params().At(i).Type()
						}
					}
					if pt == nil {
						continue
					}
					if _, isIface := pt.Underlying().(*types.Interface); !isIface {
						continue
					}
					if ptr, ok := at.(*types.Pointer); ok {
						markStructBad(structOf(ptr), true)
						if sl, ok := ptr.Elem().Underlying().(*types.Slice); ok {
							markStructBad(structOf(sl.Elem()), true)
						}
					}
				}
			}
			return true
		}
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok {
				cur = fd
			} else {
				cur = nil
			}
			ast.Inspect(d, visit)
		}
	}
}

func analyze(dir, goos string) []hit {
	fieldWrites = map[string][]fieldWrite{}
	fieldBad = map[string]bool{}
	fieldState = map[string]int{}
	scopes = map[*ast.FuncDecl]*scope{}
	funcs = map[string]*funcInfo{}
	byDecl = map[*ast.FuncDecl]*funcInfo{}
	cfg := &packages.Config{Mode: packages.LoadAllSyntax, Tests: true, Dir: dir, Env: append(os.Environ(), "GOOS="+goos, "CGO_ENABLED=0")}
	pkgs, err := packages.Load(cfg, "./internal/...", "./cmd/...")
	if err != nil {
		panic(err)
	}
	fset = pkgs[0].Fset
	for _, p := range pkgs {
		collectFieldWrites(p)
	}
	for _, p := range pkgs {
		for _, f := range p.Syntax {
			for _, d := range f.Decls {
				fd, ok := d.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				fn, _ := p.TypesInfo.Defs[fd.Name].(*types.Func)
				if fn == nil {
					continue
				}
				n := fn.Type().(*types.Signature).Results().Len()
				info := &funcInfo{decl: fd, pkg: p, results: make([]bool, n)}
				// optimistic start for string results, refined to fixpoint (greatest fixpoint)
				for i := 0; i < n; i++ {
					if b, ok := fn.Type().(*types.Signature).Results().At(i).Type().Underlying().(*types.Basic); ok && b.Kind() == types.String {
						info.results[i] = fn.Type().(*types.Signature).Results().At(i).Name() == ""
					}
				}
				if _, seen := funcs[objKey(fn)]; seen {
					continue
				}
				funcs[objKey(fn)] = info
				byDecl[fd] = info
			}
		}
	}
	// greatest fixpoint: start optimistic, demote until stable
	for changed := true; changed; {
		changed = false
		fieldState = map[string]int{}
		scopes = map[*ast.FuncDecl]*scope{}
		for _, info := range funcs {
			if !anyTrue(info.results) {
				continue
			}
			s := newScope(info.pkg, info.decl)
			ast.Inspect(info.decl.Body, func(n ast.Node) bool {
				if _, ok := n.(*ast.FuncLit); ok {
					return false
				}
				ret, ok := n.(*ast.ReturnStmt)
				if !ok {
					return true
				}
				for i := range info.results {
					if !info.results[i] {
						continue
					}
					ok := false
					if len(ret.Results) == len(info.results) {
						ok = s.trimmed(ret.Results[i])
					} else if len(ret.Results) == 1 {
						if call, isCall := ret.Results[0].(*ast.CallExpr); isCall {
							ok = s.callResultTrimmed(call, i)
						}
					}
					if !ok {
						info.results[i] = false
						changed = true
					}
				}
				return true
			})
		}
	}
	fieldState = map[string]int{}
	scopes = map[*ast.FuncDecl]*scope{}
	var hits []hit
	seenFile := map[string]bool{}
	for _, p := range pkgs {
		for fi, f := range p.Syntax {
			file := p.CompiledGoFiles[fi]
			if strings.HasSuffix(file, "_test.go") || !strings.HasSuffix(file, ".go") || seenFile[file] {
				continue
			}
			seenFile[file] = true
			src, _ := os.ReadFile(file)
			for _, d := range f.Decls {
				fd, ok := d.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				s := newScope(p, fd)
				ast.Inspect(fd.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok || len(call.Args) != 1 || !isTrimSpace(p, call) {
						return true
					}
					if s.trimmed(call.Args[0]) {
						off := func(x token.Pos) int { return p.Fset.Position(x).Offset }
						hits = append(hits, hit{file, off(call.Pos()), off(call.End()), string(src[off(call.Args[0].Pos()):off(call.Args[0].End())]), p.Fset.Position(call.Pos()).String()})
					}
					return true
				})
			}
		}
	}
	return hits
}

type hit struct {
	file  string
	s, e  int
	inner string
	pos   string
}

func main() {
	flag.Parse()
	dir := flag.Arg(0)
	var hits []hit
	counts := map[string]int{}
	goosList := []string{"linux", "darwin", "windows"}
	for _, goos := range goosList {
		for _, h := range analyze(dir, goos) {
			if counts[h.pos] == 0 {
				hits = append(hits, h)
			}
			counts[h.pos]++
		}
	}
	proven := hits[:0]
	for _, h := range hits {
		if counts[h.pos] == len(goosList) {
			proven = append(proven, h)
		}
	}
	hits = proven
	fmt.Fprintln(os.Stderr, "redundant TrimSpace proven on every GOOS:", len(hits))
	if !*apply {
		for _, h := range hits {
			fmt.Println(h.pos, strconv.Quote(h.inner))
		}
		if len(hits) > 0 {
			os.Exit(1)
		}
		return
	}
	byFile := map[string][]hit{}
	for _, h := range hits {
		byFile[h.file] = append(byFile[h.file], h)
	}
	for file, hs := range byFile {
		src, _ := os.ReadFile(file)
		sort.Slice(hs, func(i, j int) bool { return hs[i].s > hs[j].s })
		last := len(src) + 1
		for _, h := range hs {
			if h.e > last {
				continue
			}
			src = append(src[:h.s:h.s], append([]byte(h.inner), src[h.e:]...)...)
			last = h.s
		}
		os.WriteFile(file, src, 0o644)
		fmt.Println(file)
	}
}

func anyTrue(b []bool) bool {
	for _, v := range b {
		if v {
			return true
		}
	}
	return false
}
