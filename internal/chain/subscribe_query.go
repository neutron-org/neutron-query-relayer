package chain

// NOTE: usually:
// query = 'module_name.action.field=X'
//"message.module='bank'"
// query := tmquery.MustParse(fmt.Sprintf("message.module='%s'", "interchainqueries")) // todo: use types.ModuleName
const Query = "tm.event='NewBlock'"
