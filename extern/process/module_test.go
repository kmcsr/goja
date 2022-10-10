
package process_test

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kmcsr/goja"
	"github.com/kmcsr/goja/extern/require"
	"github.com/kmcsr/goja/extern/process"
)

func TestProcessEnvStructure(t *testing.T) {
	vm := goja.New()

	new(require.Registry).Enable(vm)
	process.Enable(vm)

	if c := vm.Get("process"); c == nil {
		t.Fatal("process not found")
	}

	if c, err := vm.RunString("process.env"); c == nil || err != nil {
		t.Fatal("error accessing process.env")
	}
}

func TestProcessEnvValuesArtificial(t *testing.T) {
	os.Setenv("GOJA_IS_AWESOME", "true")
	defer os.Unsetenv("GOJA_IS_AWESOME")

	vm := goja.New()

	new(require.Registry).Enable(vm)
	process.Enable(vm)

	jsRes, err := vm.RunString("process.env['GOJA_IS_AWESOME']")

	if err != nil {
		t.Fatal(fmt.Sprintf("Error executing: %s", err))
	}

	if jsRes.String() != "true" {
		t.Fatal(fmt.Sprintf("Error executing: got %s but expected %s", jsRes, "true"))
	}
}

func TestProcessEnvValuesBrackets(t *testing.T) {
	vm := goja.New()

	new(require.Registry).Enable(vm)
	process.Enable(vm)

	for _, e := range os.Environ() {
		envKeyValue := strings.SplitN(e, "=", 2)
		jsExpr := fmt.Sprintf("process.env['%s']", envKeyValue[0])

		jsRes, err := vm.RunString(jsExpr)

		if err != nil {
			t.Fatal(fmt.Sprintf("Error executing %s: %s", jsExpr, err))
		}

		if jsRes.String() != envKeyValue[1] {
			t.Fatal(fmt.Sprintf("Error executing %s: got %s but expected %s", jsExpr, jsRes, envKeyValue[1]))
		}
	}
}

func TestProcessOnerror(t *testing.T) {
	vm := goja.New()
	vm.SetFieldNameMapper(goja.UncapFieldNameMapper())
	vm.Set("log", func(msg ...any) {
		t.Log(time.Now().Format("15:04:05.000"), "console:", fmt.Sprintln(msg...))
	})

	new(require.Registry).Enable(vm)
	process.Enable(vm)

	vm.Run(func(*goja.Runtime)(err error){
		_, err = vm.RunString(`
			process.on('error', (err)=>{
				log("on error:", err);
			});
			setTimeout(()=>{ throw 'test error'; });
		`)
		return
	})
}
