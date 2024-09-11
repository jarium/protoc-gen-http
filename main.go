package main

import (
	"fmt"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"os"
)

const (
	pluginName = "github.com/jarium/protoc-gen-http"
	version    = "1.0"
)

func main() {
	protogen.Options{}.Run(func(plugin *protogen.Plugin) error {
		for _, file := range plugin.Files {
			if !file.Generate {
				continue
			}
			generateFile(plugin, file)
		}
		return nil
	})
}

func generateFile(plugin *protogen.Plugin, file *protogen.File) {
	filename := file.GeneratedFilenamePrefix + "_http.pb.go"
	g := plugin.NewGeneratedFile(filename, file.GoImportPath)

	g.P("// Code generated by protoc-gen-http. DO NOT EDIT.")
	g.P(fmt.Sprintf("// %s", pluginName))
	g.P(fmt.Sprintf("// version:%s", version))
	g.P()

	// Package declaration
	g.P("package ", file.GoPackageName)
	g.P()

	// Import necessary packages
	g.P("import (")
	g.P(`"context"`)
	g.P(`"errors"`)
	g.P(`"github.com/gin-gonic/gin"`)
	g.P(`"net/http"`)
	g.P(`"github.com/jarium/protoc-gen-http/pkg/apierror"`)
	g.P(")")

	for _, service := range file.Services {
		genService(g, service)
	}
}

func genService(g *protogen.GeneratedFile, service *protogen.Service) {
	g.P("// ", service.GoName, "HTTPServer is the HTTP server interface.")
	g.P("type ", service.GoName, "HTTPServer interface {")
	for _, method := range service.Methods {
		g.P(method.GoName, "(context.Context, *", method.Input.GoIdent, ") (*", method.Output.GoIdent, ", error)")
	}
	g.P("}")
	g.P()

	g.P("func Register", service.GoName, "HTTPServer(r *gin.Engine, srv ", service.GoName, "HTTPServer) {")
	for _, method := range service.Methods {
		httpRule := getHTTPRule(method.Desc.Options().(*descriptorpb.MethodOptions))

		if httpRule == nil {
			os.Exit(0) //no http option provided on .proto file
		}

		if getPattern := httpRule.GetGet(); getPattern != "" {
			g.P(`r.GET("`, getPattern, `", _`, service.GoName, "_", method.GoName, `_HTTP_Handler(srv))`)
		} else if postPattern := httpRule.GetPost(); postPattern != "" {
			g.P(`r.POST("`, postPattern, `", _`, service.GoName, "_", method.GoName, `_HTTP_Handler(srv))`)
		}
	}
	g.P("}")
	g.P()

	for _, method := range service.Methods {
		genMethodHandler(g, service, method)
	}
}

func genMethodHandler(g *protogen.GeneratedFile, service *protogen.Service, method *protogen.Method) {
	g.P("func _", service.GoName, "_", method.GoName, `_HTTP_Handler(srv `, service.GoName, `HTTPServer) func(c *gin.Context) {`)
	g.P("return func(c *gin.Context) {")
	g.P("var in ", method.Input.GoIdent)
	g.P("if err := c.ShouldBind(&in); err != nil {")
	g.P(`c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})`)
	g.P("return")
	g.P("}")
	g.P("out, err := srv.", method.GoName, `(c.Request.Context(), &in)`)
	g.P("if err != nil {")
	g.P("var apiErr apierror.IError")
	g.P("if errors.As(err, &apiErr) {")
	g.P("c.Error(apiErr.Unwrap())")
	g.P(`c.JSON(apiErr.GetStatusCode(), gin.H{"error": apiErr.GetMessage()})`)
	g.P("return")
	g.P("}")
	g.P("c.Error(err)")
	g.P(`c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})`)
	g.P("return")
	g.P("}")
	g.P("c.JSON(http.StatusOK, out)")
	g.P("}")
	g.P("}")
	g.P()
}

// getHTTPRule extracts the HTTP rule from the method options.
func getHTTPRule(options *descriptorpb.MethodOptions) *annotations.HttpRule {
	if ext, ok := proto.GetExtension(options, annotations.E_Http).(*annotations.HttpRule); ok {
		return ext
	}
	return nil
}
