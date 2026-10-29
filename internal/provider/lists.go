package provider

import "github.com/hashicorp/terraform-plugin-framework/types"

// strs reads an optional list of strings from the plan (nil when it is not set).
func strs(l []types.String) []string {
	out := make([]string, 0, len(l))
	for _, v := range l {
		out = append(out, v.ValueString())
	}
	return out
}

// tfStrs writes a list from the API back, keeping the attribute unset when it was unset and the API has nothing.
func tfStrs(cur []types.String, got []string) []types.String {
	if cur == nil && len(got) == 0 {
		return nil
	}
	out := make([]types.String, 0, len(got))
	for _, v := range got {
		out = append(out, types.StringValue(v))
	}
	return out
}

func sameStrs(a []types.String, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ValueString() != b[i] {
			return false
		}
	}
	return true
}

// optStr writes an optional string back from the API, keeping it unset when it was unset and the API has nothing.
func optStr(cur types.String, v string) types.String {
	if cur.IsNull() && v == "" {
		return cur
	}
	return types.StringValue(v)
}
