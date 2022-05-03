package chain

// NOTE: usually:
// query = 'module_name.action.field=X'
// query := tmquery.MustParse(fmt.Sprintf("message.module='%s'", "interchainqueries")) // todo: use types.ModuleName
const Query = "tm.event='NewBlock'"
