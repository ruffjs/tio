package shadow

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseQuery(t *testing.T) {
	cases := []struct {
		sql     string
		sel     string // select
		where   string
		orderBy string
	}{
		{
			sql: "select version from shadow",
			// add thingId automatically
			sel: "version as version, shadow.thing_id as thingId",
		},
		{
			sql: "select connected, connectedAt, disconnectedAt, remoteAddr from shadow",
			// remove status fields
			sel: "shadow.thing_id as thingId",
		},
		{
			sql: "select `tags.a`, thingId from shadow where thingId='1' order by updatedAt",
			// add alias for json path
			// add type field for json path
			sel:     "json_extract(tags, '$.a') as a, shadow.thing_id as thingId, json_type(json_extract(tags, '$.a')) as `$type_a`",
			where:   "shadow.thing_id = '1'",
			orderBy: "shadow.updated_at asc",
		},
		{
			sql:     "select * from shadow where thingId='1' order by updatedAt",
			sel:     "*",
			where:   "shadow.thing_id = '1'",
			orderBy: "shadow.updated_at asc",
		},
		{
			sql:     "select * from shadow where `state.reported.c` in ('xxx','yyy')",
			sel:     "*",
			where:   "json_extract(reported, '$.c') in ('xxx', 'yyy')",
			orderBy: ""},
		{
			sql:   "select * from shadow where `state.desired.a.b` = 1",
			sel:   "*",
			where: "json_extract(desired, '$.a.b') = 1",
		},
		{
			sql:   "select * from shadow where `tags.xx` = 'xx'",
			sel:   "*",
			where: "json_extract(tags, '$.xx') = 'xx'",
		},
		{
			sql:   "select * from shadow where `tags.xx.yy` = 1",
			sel:   "*",
			where: "json_extract(tags, '$.xx.yy') = 1",
		},

		{
			sql: "select * from shadow where `thingId`= 'abc' and (`state.desired.y` = 'xy' or `state.reported.s`='qs')" +
				" order by thingId desc, createdAt desc",
			sel:     "*",
			where:   "shadow.thing_id = 'abc' and (json_extract(desired, '$.y') = 'xy' or json_extract(reported, '$.s') = 'qs')",
			orderBy: "shadow.thing_id desc, created_at desc",
		},
		{
			sql: "select * from shadow",
			sel: "*",
		},
	}
	t.Run("test parse query", func(t *testing.T) {
		for _, v := range cases {
			res, err := parseQuerySql(v.sql)
			require.NoError(t, err, "parse sql: "+v.sql)

			require.Equal(t, v.sel, res.Select, "select sql: "+v.sql)
			require.Equal(t, v.where, res.Where, "where sql: "+v.sql)
			require.Equal(t, v.orderBy, res.OrderBy, "order by sql: "+v.sql)
		}
	})
}
