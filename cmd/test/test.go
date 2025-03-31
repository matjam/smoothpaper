package main

/*
#include <stdlib.h>

typedef unsigned char uchar;
typedef unsigned int uint;
typedef unsigned long window;
typedef void* display;

#ifdef __cplusplus
extern "C" {
#endif

int dummy_function(const unsigned char *src, const unsigned char *dst,
                   unsigned int width, unsigned int height, float speed,
                   unsigned long win, void *disp) {
  return 0;
}

#ifdef __cplusplus
}
#endif
*/
import "C"
import "fmt"

func main() {
	fmt.Println("Test")
	src := C.malloc(C.size_t(100))
	defer C.free(src)

	// Just to test if we can reference the dummy function
	_ = C.dummy_function((*C.uchar)(src), (*C.uchar)(src),
		100, 100, 1.0, 0, nil)
}
