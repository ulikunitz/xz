package nlzma

import (
	"fmt"
	"reflect"
	"testing"
)

func TestStateInitClone(t *testing.T) {
	tests := []struct {
		props Properties
	}{
		{props: Properties{LC: 3, LP: 0, PB: 2}},
		{props: Properties{LC: 4, LP: 1, PB: 3}},
		{props: Properties{LC: 5, LP: 2, PB: 4}},
	}

	for i, tc := range tests {
		t.Run(fmt.Sprintf("t=%d", i), func(t *testing.T) {
			if tc.props.verify() != nil {
				t.Fatalf("invalid properties: %#v", tc.props)
			}
			var s1, s2 state
			s1.init(tc.props)
			s2.clone(&s1)
			if !reflect.DeepEqual(s1, s2) {
				t.Fatalf("state mismatch: %#v != %#v", s1, s2)
			}
		})
	}
}
