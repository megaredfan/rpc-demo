package trace

type MetaDataCarrier struct {
	Meta *map[string]interface{}
}

func (c *MetaDataCarrier) ForeachKey(handler func(key, val string) error) error {
	for k, v := range *(c.Meta) {
		if s, ok := v.(string); ok {
			if err := handler(k, s); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *MetaDataCarrier) Set(key, val string) {
	(*c.Meta)[key] = val
}
