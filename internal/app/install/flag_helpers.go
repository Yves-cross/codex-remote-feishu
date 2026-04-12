package install

import "flag"

func flagWasProvided(flagSet *flag.FlagSet, name string) bool {
	provided := false
	flagSet.Visit(func(value *flag.Flag) {
		if value.Name == name {
			provided = true
		}
	})
	return provided
}
