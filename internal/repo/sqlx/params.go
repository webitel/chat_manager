package sqlxrepo

// named parameters set
type params map[string]interface{}

// // convertion short hand
// func (m params) map() map[string]interface{} {
// 	return (map[string]interface{})(m)
// }

// set named parameter value
func (m params) set(name string, value interface{}) {
	if _, has := m[name]; has {
		panic("params: duplicate :"+ name +" name")
	}
	m[name] = value
}