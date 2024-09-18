package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/jarium/protoc-gen-http/gen"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"path/filepath"
	"runtime"
	"text/template"
)

//go:generate protoc --go_out=example/gen --http_out=example/gen --http_opt=lib=net --proto_path=example/google --proto_path=example example/gen/example_pb/example.proto

const (
	pluginName = "github.com/jarium/protoc-gen-http"
	version    = "1.1"
)

var (
	lib       *string
	templates = map[string]string{
		"net": "gen/net.tmpl",
		"gin": "gen/gin.tmpl",
	}
	selectedTemplate string
)

func main() {
	var flags flag.FlagSet
	lib = flags.String("lib", "", "http lib that will be used for generated code")

	protogen.Options{
		ParamFunc: flags.Set,
	}.Run(func(plugin *protogen.Plugin) error {
		if *lib == "" {
			*lib = "net"
		}

		t, ok := templates[*lib]

		if !ok {
			var libNames string
			for n := range templates {
				libNames += n + ","
			}

			libNames = libNames[:len(libNames)-1]
			return fmt.Errorf("invalid lib name provided, valid lib names: %s", libNames)
		}

		selectedTemplate = t

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
	services := getHttpServices(file.Services)

	if len(services) == 0 {
		return //no service has http option
	}

	templateData := gen.TemplateData{
		Entrance: "Code generated by protoc-gen-http. DO NOT EDIT.",
		GoMod:    pluginName,
		Version:  version,
		Package:  string(file.GoPackageName),
		Services: services,
	}

	_, cFile, _, ok := runtime.Caller(0)

	if !ok {
		panic("unable to get file location of caller")
	}

	layoutFile := filepath.Join(filepath.Dir(cFile), "gen/layout.tmpl")
	templateFile := filepath.Join(filepath.Dir(cFile), selectedTemplate)

	tmpl, err := template.ParseFiles(layoutFile, templateFile)

	if err != nil {
		panic(err)
	}

	var content bytes.Buffer
	err = tmpl.Execute(&content, templateData)

	if err != nil {
		panic(err)
	}

	generatedFile := plugin.NewGeneratedFile(file.GeneratedFilenamePrefix+"_http.pb.go", file.GoImportPath)
	_, err = generatedFile.Write(content.Bytes())

	if err != nil {
		panic(err)
	}
}

// getHttpServices returns the http services data with their methods that has http options
func getHttpServices(ps []*protogen.Service) []gen.Service {
	var data []gen.Service

	for _, service := range ps {
		sd := gen.Service{
			Name: service.GoName,
		}
		for _, method := range service.Methods {
			if method.Desc.IsStreamingClient() || method.Desc.IsStreamingServer() {
				continue
			}

			rule, ok := proto.GetExtension(method.Desc.Options(), annotations.E_Http).(*annotations.HttpRule)
			if rule != nil && ok {
				var uri string
				var requestMethod string

				if u := rule.GetGet(); u != "" {
					requestMethod = "GET"
					uri = u
				} else if u := rule.GetPost(); u != "" {
					requestMethod = "POST"
					uri = u
				}

				sd.Methods = append(sd.Methods, gen.Method{
					Name:          method.GoName,
					Uri:           uri,
					RequestMethod: requestMethod,
					In:            method.Input.GoIdent.GoName,
					Out:           method.Output.GoIdent.GoName,
				})
			}
		}

		if len(sd.Methods) > 0 {
			data = append(data, sd)
		}
	}

	return data
}
