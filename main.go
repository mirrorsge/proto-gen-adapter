package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"text/template"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/golang/protobuf/protoc-gen-go/generator"
)

// 定义模块
const tmplService = `
{{$root := .}}
import (
	"github.com/mirrorsge/grpc-lb/registry/consul"
	"github.com/mirrorsge/grpc-lb/balancer"
	"google.golang.org/grpc"
)

var ServiceName = {{.ServiceName}}
var {{.ServiceName}}Client *grpc.ClientConn

func init() {
	consul.RegisterResolver("consul", &consulapi.Config{Address: "http://59.110.162.134:8500"}, ServiceName)
	conn, err := grpc.Dial("consul:///", grpc.WithInsecure(), grpc.WithBalancerName(balancer.RoundRobin))
	{{.ServiceName}}Client = proto.NewAlphaClient(conn)
}


{{range $_, $m := .MethodList}}
func (p *{{$root.ServiceName}}Client) {{$m.MethodName}}(
	ctx context.Context, in *{{$m.InputTypeName}}
) (out *{{$m.OutputTypeName}},error) {
	return p.{{$m.MethodName}}(ctx,in)
}
{{end}}
`

// 定义服务和接口描述结构
type ServiceSpec struct {
	ServiceName string
	MethodList  []ServiceMethodSpec
}

type ServiceMethodSpec struct {
	MethodName     string
	InputTypeName  string
	OutputTypeName string
}

// 解析每个服务的ServiceSpec元信息
func (p *rpcAdapterPlugin) buildServiceSpec(svc *descriptor.ServiceDescriptorProto) *ServiceSpec {
	spec := &ServiceSpec{ServiceName: generator.CamelCase(svc.GetName())}

	for _, m := range svc.Method {
		spec.MethodList = append(spec.MethodList, ServiceMethodSpec{
			MethodName:     generator.CamelCase(m.GetName()),
			InputTypeName:  p.TypeName(p.ObjectNamed(m.GetInputType())),
			OutputTypeName: p.TypeName(p.ObjectNamed(m.GetOutputType())),
		})
	}

	return spec
}

// 自定义方法，生成导入代码
func (p *rpcAdapterPlugin) genServiceCode(svc *descriptor.ServiceDescriptorProto) {
	spec := p.buildServiceSpec(svc)

	var buf bytes.Buffer
	t := template.Must(template.New("").Parse(tmplService))
	err := t.Execute(&buf, spec)
	if err != nil {
		log.Fatal(err)
	}

	p.P(buf.String())
}

// 定义netrpcPlugin类，generator 作为成员变量存在, 继承公有方法
type rpcAdapterPlugin struct{ *generator.Generator }

// 返回插件名称
func (p *rpcAdapterPlugin) Name() string {
	return "rpcadapter"
}

// 通过g 进入初始化
func (p *rpcAdapterPlugin) Init(g *generator.Generator) {
	p.Generator = g
}

// 生成导入包
func (p *rpcAdapterPlugin) GenerateImports(file *generator.FileDescriptor) {
	if len(file.Service) > 0 {
		p.genImportCode(file)
	}
}

// 生成主体代码
func (p *rpcAdapterPlugin) Generate(file *generator.FileDescriptor) {
	for _, svc := range file.Service {
		p.genServiceCode(svc)
	}
}

// 自定义方法，生成导入包
func (p *rpcAdapterPlugin) genImportCode(file *generator.FileDescriptor) {
	p.P("// TODO: import code here")
	p.P(`import "net/rpc"`)
}

// 自定义方法，生成导入代码
/*
func (p *netrpcPlugin) genServiceCode(svc *descriptor.ServiceDescriptorProto) {
	p.P("// TODO: service code, Name = " + svc.GetName())
}
*/

// 注册插件
func init() {
	generator.RegisterPlugin(new(rpcAdapterPlugin))
}

// 以下内容都来自protoc-gen-go/main.go
func main() {
	g := generator.New()

	data, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		g.Error(err, "reading input")
	}

	if err := proto.Unmarshal(data, g.Request); err != nil {
		g.Error(err, "parsing input proto")
	}

	if len(g.Request.FileToGenerate) == 0 {
		g.Fail("no files to generate")
	}

	g.CommandLineParameters(g.Request.GetParameter())

	// Create a wrapped version of the Descriptors and EnumDescriptors that
	// point to the file that defines them.
	g.WrapTypes()

	g.SetPackageNames()
	g.BuildTypeNameMap()

	g.GenerateAllFiles()

	// Send back the results.
	data, err = proto.Marshal(g.Response)
	if err != nil {
		g.Error(err, "failed to marshal output proto")
	}
	_, err = os.Stdout.Write(data)
	if err != nil {
		g.Error(err, "failed to write output proto")
	}
}