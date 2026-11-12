// Package vm executes bytecode modules on a stack-based virtual machine.
// Output is written to a host-injected writer; runtime errors are returned
// as structured values, never as panics.
//
// Allowed Tread imports: source, bytecode.
package vm
