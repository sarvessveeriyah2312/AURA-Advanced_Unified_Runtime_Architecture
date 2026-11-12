package bytecode

import "fmt"

// Verify runs Validate and then the stack verifier (ADR-0011 §2). The
// verifier is a data-flow pass over each function's reachable
// instructions, using the stack effects of docs/05 §5.3. It proves that:
//
//   - no instruction pops more values than the frame's operand stack
//     holds;
//   - every instruction is reached with the same stack depth from every
//     predecessor;
//   - execution cannot run past the end of the code.
//
// Value tags are not tracked; the VM still checks them dynamically.
// Unreachable instructions are not analyzed: the VM never executes them.
func (m *Module) Verify() error {
	if err := m.Validate(); err != nil {
		return err
	}
	for fi := range m.Functions {
		if err := m.verifyFunction(&m.Functions[fi]); err != nil {
			return err
		}
	}
	return nil
}

// stackEffect returns how many values in executes pops and pushes.
func (m *Module) stackEffect(in Instr) (pop, push int) {
	switch in.Op {
	case OpConst, OpUnit, OpLoadLocal:
		return 0, 1
	case OpPop, OpStoreLocal, OpJumpIfFalse, OpReturn:
		return 1, 0
	case OpJump:
		return 0, 0
	case OpNegInt, OpNegFloat, OpNot,
		OpPrintInt, OpPrintFloat, OpPrintString, OpPrintBool,
		OpToStringInt, OpToStringFloat, OpToStringBool:
		return 1, 1
	case OpBuildString:
		return in.A, 1
	case OpCall:
		return m.Functions[in.A].NumParams, 1
	}
	return 2, 1 // binary arithmetic, concatenation and comparisons
}

func (m *Module) verifyFunction(fn *Function) error {
	depth := make([]int, len(fn.Code)) // stack depth before each pc; -1 = not reached
	for i := range depth {
		depth[i] = -1
	}
	bad := func(pc int, format string, args ...any) error {
		return &ValidationError{Function: fn.Name, PC: pc, Msg: fmt.Sprintf(format, args...)}
	}
	work := []int{0}
	depth[0] = 0
	for len(work) > 0 {
		pc := work[len(work)-1]
		work = work[:len(work)-1]
		in := fn.Code[pc]
		pop, push := m.stackEffect(in)
		if depth[pc] < pop {
			return bad(pc, "%s needs %d stack values, has %d", in.Op, pop, depth[pc])
		}
		after := depth[pc] - pop + push

		var next []int
		switch in.Op {
		case OpReturn:
			// ends the frame
		case OpJump:
			next = []int{in.A}
		case OpJumpIfFalse:
			next = []int{pc + 1, in.A}
		default:
			next = []int{pc + 1}
		}
		for _, n := range next {
			if n >= len(fn.Code) {
				return bad(pc, "execution can run past the end of the code")
			}
			switch depth[n] {
			case -1:
				depth[n] = after
				work = append(work, n)
			case after:
			default:
				return bad(n, "reached with stack depth %d and %d", depth[n], after)
			}
		}
	}
	return nil
}
