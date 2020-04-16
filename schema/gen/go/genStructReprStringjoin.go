package gengo

import (
	"io"

	"github.com/ipld/go-ipld-prime/schema"
	"github.com/ipld/go-ipld-prime/schema/gen/go/mixins"
)

var _ TypeGenerator = &structReprStringjoinGenerator{}

func NewStructReprStringjoinGenerator(pkgName string, typ schema.TypeStruct, adjCfg *AdjunctCfg) TypeGenerator {
	return structReprStringjoinGenerator{
		structGenerator{
			adjCfg,
			mixins.MapTraits{
				pkgName,
				string(typ.Name()),
				adjCfg.TypeSymbol(typ),
			},
			pkgName,
			typ,
		},
	}
}

type structReprStringjoinGenerator struct {
	structGenerator
}

func (g structReprStringjoinGenerator) GetRepresentationNodeGen() NodeGenerator {
	return structReprStringjoinReprGenerator{
		g.AdjCfg,
		mixins.StringTraits{
			g.PkgName,
			string(g.Type.Name()) + ".Repr",
			"_" + g.AdjCfg.TypeSymbol(g.Type) + "__Repr",
		},
		g.PkgName,
		g.Type,
	}
}

type structReprStringjoinReprGenerator struct {
	AdjCfg *AdjunctCfg
	mixins.StringTraits
	PkgName string
	Type    schema.TypeStruct
}

func (g structReprStringjoinReprGenerator) EmitNodeType(w io.Writer) {
	// The type is structurally the same, but will have a different set of methods.
	doTemplate(`
		type _{{ .Type | TypeSymbol }}__Repr _{{ .Type | TypeSymbol }}
	`, w, g.AdjCfg, g)
}

func (g structReprStringjoinReprGenerator) EmitNodeTypeAssertions(w io.Writer) {
	doTemplate(`
		var _ ipld.Node = &_{{ .Type | TypeSymbol }}__Repr{}
	`, w, g.AdjCfg, g)
}

func (g structReprStringjoinReprGenerator) EmitNodeMethodAsString(w io.Writer) {
	// Prerequisites:
	//  - every field must be a string, or have string representation.
	//    - this should've been checked when compiling the type system info.
	//    - we're willing to imply a base-10 atoi/itoa for ints (but it's not currently supported).
	//  - there are NO sanity checks that your value doesn't contain the delimiter
	//    - you need to do this in validation hooks or some other way
	//  - optional or nullable fields are not supported with this representation strategy.
	//    - this should've been checked when compiling the type system info.
	//    - if support for this is added in the future, you can bet all optionals
	//      will be required to be *either* in a row at the start, or in a row at the end.
	//      (a 'direction' property might also be needed, so behavior is defined if every field is optional.)
	doTemplate(`
		func (n *_{{ .Type | TypeSymbol }}__Repr) AsString() (string, error) {
			return {{ "" }}
			{{- $type := .Type -}} {{- /* ranging modifies dot, unhelpfully */ -}}
			{{- range $i, $field := .Type.Fields }}
			{{- if $i }} + "{{ $type.RepresentationStrategy.GetDelim }}" + {{end -}}
			(*_{{ $field.Type | TypeSymbol }}__Repr)(&n.{{ $field | FieldSymbolLower }}).String()
			{{- end}}, nil
		}
	`, w, g.AdjCfg, g)
}

func (g structReprStringjoinReprGenerator) EmitNodeMethodStyle(w io.Writer) {
	// REVIEW: this appears to be standard even across kinds; can we extract it?
	doTemplate(`
		func (_{{ .Type | TypeSymbol }}__Repr) Style() ipld.NodeStyle {
			return _{{ .Type | TypeSymbol }}__ReprStyle{}
		}
	`, w, g.AdjCfg, g)
}

func (g structReprStringjoinReprGenerator) EmitNodeStyleType(w io.Writer) {
	// REVIEW: this appears to be standard even across kinds; can we extract it?
	doTemplate(`
		type _{{ .Type | TypeSymbol }}__ReprStyle struct{}

		func (_{{ .Type | TypeSymbol }}__ReprStyle) NewBuilder() ipld.NodeBuilder {
			var nb _{{ .Type | TypeSymbol }}__ReprBuilder
			nb.Reset()
			return &nb
		}
	`, w, g.AdjCfg, g)
}

// --- NodeBuilder and NodeAssembler --->

func (g structReprStringjoinReprGenerator) GetNodeBuilderGenerator() NodeBuilderGenerator {
	return structReprStringjoinReprBuilderGenerator{
		g.AdjCfg,
		mixins.StringAssemblerTraits{
			g.PkgName,
			g.TypeName,
			"_" + g.AdjCfg.TypeSymbol(g.Type) + "__Repr",
		},
		g.PkgName,
		g.Type,
	}
}

type structReprStringjoinReprBuilderGenerator struct {
	AdjCfg *AdjunctCfg
	mixins.StringAssemblerTraits
	PkgName string
	Type    schema.TypeStruct
}

func (g structReprStringjoinReprBuilderGenerator) EmitNodeBuilderType(w io.Writer) {
	doTemplate(`
		type _{{ .Type | TypeSymbol }}__ReprBuilder struct {
			_{{ .Type | TypeSymbol }}__ReprAssembler
		}
	`, w, g.AdjCfg, g)
}
func (g structReprStringjoinReprBuilderGenerator) EmitNodeBuilderMethods(w io.Writer) {
	doTemplate(`
		func (nb *_{{ .Type | TypeSymbol }}__ReprBuilder) Build() ipld.Node {
			return nb.w
		}
		func (nb *_{{ .Type | TypeSymbol }}__ReprBuilder) Reset() {
			var w _{{ .Type | TypeSymbol }}
			*nb = _{{ .Type | TypeSymbol }}__ReprBuilder{_{{ .Type | TypeSymbol }}__ReprAssembler{w: &w, fcb:nb.fcb_root}}
		}
		func (nb *_{{ .Type | TypeSymbol }}__ReprBuilder) fcb_root() error {
			if nb.z == true {
				return mixins.StringAssembler{"{{ .PkgName }}.{{ .TypeName }}.Repr"}.AssignNull()
			}
			return nil
		}
	`, w, g.AdjCfg, g)
	// Generate a single-step construction function -- this is easy to do for a scalar,
	//  and all representations of scalar kind can be expected to have a method like this.
	// The function is attached to the nodestyle for convenient namespacing;
	//  it needs no new memory, so it would be inappropriate to attach to the builder or assembler.
	// The function is directly used internally by anything else that might involve recursive destructuring on the same scalar kind
	//  (for example, structs using stringjoin strategies that have one of this type as a field, etc).
	// Since we're a representation of scalar kind, and can recurse,
	//  we ourselves presume this plain construction method must also exist for all our members.
	// REVIEW: We could make an immut-safe verion of this and export it on the NodeStyle too, as `FromString(string)`.
	// FUTURE: should engage validation flow.
	doTemplate(`
		func (_{{ .Type | TypeSymbol }}__ReprStyle) construct(w *_{{ .Type | TypeSymbol }}, v string) error {
			ss, err := mixins.SplitExact(v, "{{ .Type.RepresentationStrategy.GetDelim }}", {{ len .Type.Fields }})
			if err != nil {
				return ipld.ErrUnmatchable{TypeName:"{{ .PkgName }}.{{ .Type.Name }}.Repr", Reason: err}
			}
			{{- $dot := . -}} {{- /* ranging modifies dot, unhelpfully */ -}}
			{{- range $i, $field := .Type.Fields }}
			if err := (_{{ $field.Type | TypeSymbol }}__ReprStyle{}).construct(&w.{{ $field | FieldSymbolLower }}, ss[{{ $i }}]); err != nil {
				return ipld.ErrUnmatchable{TypeName:"{{ $dot.PkgName }}.{{ $dot.Type.Name }}.Repr", Reason: err}
			}
			{{- end}}
			return nil
		}
	`, w, g.AdjCfg, g)
}
func (g structReprStringjoinReprBuilderGenerator) EmitNodeAssemblerType(w io.Writer) {
	// - 'w' is the "**w**ip" pointer.
	// - 'z' is used to denote a null (in case we're used in a context that's acceptable).  z for **z**ilch.
	// - 'fcb' is the **f**inish **c**all**b**ack, supplied by the parent if we're a child assembler.
	doTemplate(`
		type _{{ .Type | TypeSymbol }}__ReprAssembler struct {
			w *_{{ .Type | TypeSymbol }}
			z bool
			fcb func() error
		}
	`, w, g.AdjCfg, g)
}
func (g structReprStringjoinReprBuilderGenerator) EmitNodeAssemblerMethodAssignNull(w io.Writer) {
	// FIXME questions about if a state machine might be necessary here.
	//  This shouldn't be allowed if AssignString was previously used.
	//  If 'fcb' is always assigned at start, a nil there can perhaps do double duty?
	doTemplate(`
		func (na *_{{ .Type | TypeSymbol }}__ReprAssembler) AssignNull() error {
			na.z = true
			return na.fcb()
		}
	`, w, g.AdjCfg, g)
}
func (g structReprStringjoinReprBuilderGenerator) EmitNodeAssemblerMethodAssignString(w io.Writer) {
	// This method contains a branch to support MaybeUsesPtr because new memory may need to be allocated.
	//  This allocation only happens if the 'w' ptr is nil, which means we're being used on a Maybe;
	//  otherwise, the 'w' ptr should already be set, and we fill that memory location without allocating, as usual.
	doTemplate(`
		func (na *_{{ .Type | TypeSymbol }}__ReprAssembler) AssignString(v string) error {
			{{- if .Type | MaybeUsesPtr }}
			if na.w == nil {
				na.w = &_{{ .Type | TypeSymbol }}{}
			}
			{{- end}}
			if err := (_{{ .Type | TypeSymbol }}__ReprStyle{}).construct(na.w, v); err != nil {
				return err
			}
			return na.fcb()
		}
	`, w, g.AdjCfg, g)
}

func (g structReprStringjoinReprBuilderGenerator) EmitNodeAssemblerMethodAssignNode(w io.Writer) {
	doTemplate(`
		func (na *_{{ .Type | TypeSymbol }}__ReprAssembler) AssignNode(v ipld.Node) error {
			if v.IsNull() {
				return na.AssignNull()
			}
			if v2, err := v.AsString(); err != nil {
				return err
			} else {
				return na.AssignString(v2)
			}
		}
	`, w, g.AdjCfg, g)
}
func (g structReprStringjoinReprBuilderGenerator) EmitNodeAssemblerOtherBits(w io.Writer) {
	// None for this.
}