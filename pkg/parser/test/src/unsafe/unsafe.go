package unsafe

import "unsafe"

func MockFunction() {
	u := unsafe.Pointer(nil)
	offset := unsafe.Sizeof(0)
	p := unsafe.Pointer(uintptr(u) + offset)

	if p != nil {
		print("Pointer is not nil")
	} else {
		print("Pointer is nil")
	}
	if uintptr(p) == 0 {
		print("Pointer is zero")
	} else {
		print("Pointer is not zero")
	}
	if uintptr(p) > 0 {
		print("Pointer is greater than zero")
	} else {
		print("Pointer is not greater than zero")
	}
	if uintptr(p) < 0 {
		print("Pointer is less than zero")
	} else {
		print("Pointer is not less than zero")
	}
}
