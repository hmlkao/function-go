package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"

	"github.com/crossplane/function-sdk-go/errors"
	"github.com/crossplane/function-sdk-go/logging"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/response"
	"github.com/hmlkao/function-go/input/v1beta1"
	"google.golang.org/protobuf/types/known/structpb"
)

// Function returns whatever response you ask it to.
type Function struct {
	fnv1.UnimplementedFunctionRunnerServiceServer

	log logging.Logger
}

const (
	cacheDir = ".cache"
)

// RunFunction runs the Function.
func (f *Function) RunFunction(ctx context.Context, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	f.log.Debug("Running Function", "tag", req.GetMeta().GetTag())

	rsp := response.To(req, response.DefaultTTL)

	script := &v1beta1.Script{}
	if err := request.GetInput(req, script); err != nil {
		// You can set a custom status condition on the claim. This allows you to
		// communicate with the user. See the link below for status condition
		// guidance.
		// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties
		response.ConditionFalse(rsp, "FunctionSuccess", "InternalError").
			WithMessage("Something went wrong.").
			TargetCompositeAndClaim()

		// You can emit an event regarding the claim. This allows you to communicate
		// with the user. Note that events should be used sparingly and are subject
		// to throttling; see the issue below for more information.
		// https://github.com/crossplane/crossplane/issues/5802
		response.Warning(rsp, errors.New("something went wrong")).
			TargetCompositeAndClaim()

		response.Fatal(rsp, errors.Wrapf(err, "cannot get Function input from %T", req))
		return rsp, nil
	}

	// Create cache directory to store temporary data
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot create cache directory %s", cacheDir))
		return rsp, nil
	}
	defer func() {
		// Remove all cached files
		_ = os.RemoveAll(cacheDir)
	}()

	// Store input script as a file
	var inputScript string
	switch {
	case script.Path != "":
		if _, err := os.Stat(script.Path); os.IsNotExist(err) {
			response.Fatal(rsp, errors.Wrapf(err, "input script path %s does not exist", script.Path))
			return rsp, nil
		}
		inputScript = script.Path
	case script.Inline != "":
		inputScript = cacheDir + "/inputscript.go"
		if err := os.WriteFile(inputScript, []byte(script.Inline), 0644); err != nil {
			response.Fatal(rsp, errors.Wrapf(err, "cannot write input script to file"))
			return rsp, nil
		}
	default:
		response.Fatal(rsp, errors.New("Path or Inline script must be provided"))
	}

	// Store request context as a file
	contextBytes, err := json.Marshal(req.GetContext())
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot marshal request context to JSON"))
		return rsp, nil
	}
	if err := os.WriteFile(cacheDir+"/context.json", contextBytes, 0644); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot write context to file"))
		return rsp, nil
	}

	// Store observed resource as a file
	observedBytes, err := json.Marshal(req.GetObserved())
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot marshal observed resource to JSON"))
		return rsp, nil
	}
	if err := os.WriteFile(cacheDir+"/observed.json", observedBytes, 0644); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot write observed resource to file"))
		return rsp, nil
	}

	// Run the input script
	cmd := exec.CommandContext(ctx, "go", "run", inputScript)
	outputByte, err := cmd.CombinedOutput()
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot run input script: %s", string(outputByte)))
		return rsp, nil
	}
	f.log.Debug("Input script output", "output", string(outputByte))

	// Parse the output as JSON and store it in the response context under the "output" key
	output := string(outputByte)
	var parsed map[string]any
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot parse output as JSON"))
		return rsp, nil
	}
	st, err := structpb.NewStruct(parsed)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot convert parsed JSON to structpb.Struct"))
		return rsp, nil
	}

	// Store output in the response context
	response.SetContextKey(rsp, "stepID", structpb.NewStructValue(st))

	return rsp, nil
}
