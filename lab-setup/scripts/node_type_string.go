// Code generated by "stringer -type=node_type"; DO NOT EDIT.

package main

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[UNDEFINED-0]
	_ = x[ALL-1]
	_ = x[MASTER-2]
	_ = x[WORKER-3]
}

const _node_type_name = "UNDEFINEDALLMASTERWORKER"

var _node_type_index = [...]uint8{0, 9, 12, 18, 24}

func (i node_type) String() string {
	if i >= node_type(len(_node_type_index)-1) {
		return "node_type(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _node_type_name[_node_type_index[i]:_node_type_index[i+1]]
}