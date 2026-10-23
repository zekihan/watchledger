package anidb

import "strconv"

type Credit struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Character   string `json:"character,omitempty"`
	Job         string `json:"job,omitempty"`
	ProfilePath string `json:"profile_path,omitempty"`
	Source      string `json:"source"`
}
type CreditSet struct {
	Cast []Credit `json:"cast"`
	Crew []Credit `json:"crew"`
}

func positiveIdentity(raw string) (int64, bool) {
	id, err := strconv.ParseInt(raw, 10, 64)
	return id, err == nil && id > 0 && strconv.FormatInt(id, 10) == raw
}
func Credits(a Anime) CreditSet {
	out := CreditSet{Cast: []Credit{}, Crew: []Credit{}}
	cast := map[int64]int{}
	roles := map[string]bool{}
	for _, character := range a.Characters {
		for _, voice := range character.Voices {
			id, ok := positiveIdentity(voice.ID)
			if !ok || voice.Name == "" {
				continue
			}
			role := voice.ID + ":" + character.Name
			if roles[role] {
				continue
			}
			roles[role] = true
			if index, exists := cast[id]; exists {
				out.Cast[index].Character += ", " + character.Name
				continue
			}
			cast[id] = len(out.Cast)
			out.Cast = append(out.Cast, Credit{ID: id, Name: voice.Name, Character: character.Name, ProfilePath: PosterURL(voice.Picture), Source: "anidb"})
		}
	}
	for _, creator := range a.Creators {
		id, ok := positiveIdentity(creator.ID)
		if !ok || creator.Name == "" {
			continue
		}
		role := creator.ID + ":" + creator.Type
		if roles["crew:"+role] {
			continue
		}
		roles["crew:"+role] = true
		out.Crew = append(out.Crew, Credit{ID: id, Name: creator.Name, Job: creator.Type, Source: "anidb"})
	}
	return out
}
