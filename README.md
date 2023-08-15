# goHookTool

基于toolexec的golang hook工具

## 获取

- 编译: `go build ./cmd/autobuild`
- 直接下载安装: `go install github.com/ListenOcean/goHookTool/cmd/autobuild@latest`

## 使用

- 指定编译使用的 go 可执行文件位置（默认从 PATH 中搜索）：`export CUSTOMGOBIN=/data/home/user/sdk/go1.18/bin/go`
- 指定 Hook 配置文件（默认为空，即不做任何 Hook）：`export CUSTOMCONFIG=/data/home/user/hooktool/configs/config.yaml`

```bash
autobuild build ./examples/example1
```

## 配置

配置文件格式

```yaml
hookpoints:
  pkgName1:
    - funcName1
    - funcName2
  pkgName2:
    - funcName3
    - funcName4
codes:
  funcName1:
    epilog: |
      custom line1
      custom line2
    prolog: |
      custom line3
```

配置文件示例

```yaml
hookpoints:
  encoding/json:
    - encoding/json.Unmarshal
    - encoding/json.NewDecoder
    - encoding/json.(*Decoder).Decode
  runtime:
    - runtime.concatstrings
  fmt:
    - fmt.Sprintf
  bytes:
    - bytes.NewBuffer
    - bytes.NewReader
  strings:
    - strings.Join
    - strings.Repeat
    - strings.Replace
  os:
    - os.OpenFile
    - os.Remove
    - os.Rename
  os/exec:
    - os/exec.Command
    - os/exec.(*Cmd).Start
  net/http:
    - net/http.(*Client).Do
    - net/http.(*Request).FormValue

codes:
  runtime.concatstrings:
    # 如果不需要修改可不填(不填会默认)
    prolog: |
      _epilog, _prolog_abort_err := (*_prolog)(buf, a)
    # epilog 解决 返回值逃逸的问题
    epilog: |
      buf = nil
      defer func() { _epilog(_result0) }()    
```
