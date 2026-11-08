package project

import "encoding/json"

// OptionCase 是一个选项分支（PI v2 的 option.cases[]）。
type OptionCase struct {
	Name             string         `json:"name"`
	PipelineOverride map[string]any `json:"pipeline_override"`
}

// Option 是一个可配置项（PI v2 的 option[]）。
type Option struct {
	Name  string       `json:"name"`
	Label string       `json:"label"`
	Cases []OptionCase `json:"cases"`
}

// parseOptions 兼容两种写法：`"option": [ ... ]` 与 `"option": { "名字": { ... } }`。
// 本项目 tasks/*.json 用的是后者（import 片段按名字建索引）。
func parseOptions(raw json.RawMessage) ([]Option, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var asList []Option
	if err := json.Unmarshal(raw, &asList); err == nil {
		return asList, nil
	}
	var asMap map[string]Option
	if err := json.Unmarshal(raw, &asMap); err != nil {
		return nil, err
	}
	out := make([]Option, 0, len(asMap))
	for name, o := range asMap {
		if o.Name == "" {
			o.Name = name
		}
		out = append(out, o)
	}
	return out, nil
}

// FindOption 按名字找选项定义。
func (p *Interface) FindOption(name string) (*Option, bool) {
	for i := range p.Option {
		if p.Option[i].Name == name {
			return &p.Option[i], true
		}
	}
	return nil, false
}

// BuildOverride 合成最终传给 MaaTasker 的 pipeline_override：
// 先铺任务自身的 pipeline_override，再按 MXU 里的选择叠加各选项分支的覆盖。
//
// 合并规则：同名节点深合并；标量/数组由**后合并者**（选项分支）覆盖。
// 这个优先级是推断的（任务默认值先铺、用户显式选择更具体），
// 未与 MXU 逐键比对；若发现某选项不生效，先查这里。
func BuildOverride(task Task, opts []Option, selected map[string][]string) map[string]any {
	out := map[string]any{}
	deepMerge(out, task.PipelineOverride)

	for _, opt := range opts {
		cases, ok := selected[opt.Name]
		if !ok {
			continue
		}
		for _, want := range cases {
			for _, c := range opt.Cases {
				if c.Name != want {
					continue
				}
				deepMerge(out, c.PipelineOverride)
				break
			}
		}
	}
	return out
}

// deepMerge 把 src 深合并进 dst，同名键递归，标量与数组直接覆盖。
func deepMerge(dst, src map[string]any) {
	for k, sv := range src {
		if sm, ok := sv.(map[string]any); ok {
			if dm, ok := dst[k].(map[string]any); ok {
				deepMerge(dm, sm)
				continue
			}
			cp := map[string]any{}
			deepMerge(cp, sm)
			dst[k] = cp
			continue
		}
		dst[k] = sv
	}
}
