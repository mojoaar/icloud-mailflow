package web

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/mojoaar/icloud-mailflow/internal/db"
)

func rulesListHandler(repo *db.RulesRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rules, err := repo.List()
		if err != nil {
			rules = []db.Rule{}
		}
		data := map[string]any{"Rules": rules}
		if r.Header.Get("HX-Request") == "true" {
			renderPartial(w, "rules_list", data)
			return
		}
		renderPage(w, r, "Rules", "rules_list", data)
	}
}

func rulesNewHandler(foldersRepo *db.FoldersRepo, contactsRepo *db.ContactsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		folders, _ := foldersRepo.List()
		contacts, _ := contactsRepo.ListAll()
		data := map[string]any{"Rule": &db.Rule{Enabled: true}, "New": true, "Fields": conditionFields(), "Folders": folders, "Contacts": contacts}
		renderPage(w, r, "New Rule", "rules_form", data)
	}
}

func rulesCreateHandler(repo *db.RulesRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		rule := &db.Rule{
			Name:        r.FormValue("name"),
			Description: r.FormValue("description"),
			Enabled:     r.FormValue("enabled") == "on",
		}
		rule.Priority, _ = strconv.Atoi(r.FormValue("priority"))
		if rule.Priority == 0 {
			rules, _ := repo.List()
			for _, r := range rules {
				if r.Name != "_catch_all" {
					rule.Priority++
				}
			}
		}
		parseConditions(r, rule)
		parseActions(r, rule)
		if err := repo.Create(rule); err != nil {
			renderPage(w, r, "New Rule", "rules_form", map[string]any{"Rule": rule, "Error": err.Error(), "New": true, "Fields": conditionFields(), "Folders": []db.Folder{}, "Contacts": []db.Contact{}})
			return
		}
		repo.EnsureCatchAll()
		http.Redirect(w, r, "/rules", http.StatusSeeOther)
	}
}

func rulesEditHandler(repo *db.RulesRepo, foldersRepo *db.FoldersRepo, contactsRepo *db.ContactsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		rule, err := repo.Get(id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		folders, _ := foldersRepo.List()
		contacts, _ := contactsRepo.ListAll()
		renderPage(w, r, "Edit Rule", "rules_form", map[string]any{"Rule": rule, "Edit": true, "Fields": conditionFields(), "Folders": folders, "Contacts": contacts})
	}
}

func rulesUpdateHandler(repo *db.RulesRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		r.ParseForm()
		rule, err := repo.Get(id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		rule.Name = r.FormValue("name")
		rule.Description = r.FormValue("description")
		rule.Enabled = r.FormValue("enabled") == "on"
		rule.Priority, _ = strconv.Atoi(r.FormValue("priority"))
		parseConditions(r, rule)
		parseActions(r, rule)
		if err := repo.Update(rule); err != nil {
			renderPage(w, r, "Edit Rule", "rules_form", map[string]any{"Rule": rule, "Error": err.Error(), "Edit": true, "Fields": conditionFields()})
			return
		}
		repo.EnsureCatchAll()
		http.Redirect(w, r, "/rules", http.StatusSeeOther)
	}
}

func rulesDeleteConfirmHandler(repo *db.RulesRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		rule, err := repo.Get(id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		renderPage(w, r, "Delete Rule", "rules_delete", map[string]any{"Rule": rule})
	}
}

func rulesDeleteHandler(repo *db.RulesRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		repo.Delete(id)
		if r.Header.Get("HX-Request") == "true" {
			rulesListHandler(repo)(w, r)
			return
		}
		http.Redirect(w, r, "/rules", http.StatusSeeOther)
	}
}

func rulesReorderHandler(repo *db.RulesRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		var ids []int64
		for _, v := range r.Form["rule_ids"] {
			id, _ := strconv.ParseInt(v, 10, 64)
			ids = append(ids, id)
		}
		repo.Reorder(ids)
		rulesListHandler(repo)(w, r)
	}
}

func conditionFields() []map[string]string {
	return []map[string]string{
		{"value": "from", "label": "From"},
		{"value": "to", "label": "To"},
		{"value": "cc", "label": "CC"},
		{"value": "subject", "label": "Subject"},
		{"value": "has_attachment", "label": "Has Attachment"},
	}
}

func parseConditions(r *http.Request, rule *db.Rule) {
	rule.Groups = nil
	r.ParseForm()
	fields := r.Form["cond_field"]
	ops := r.Form["cond_op"]
	vals := r.Form["cond_value"]
	if len(fields) == 0 {
		return
	}
	group := db.ConditionGroup{Operator: "AND"}
	for i := range fields {
		if i < len(ops) && i < len(vals) {
			group.Conditions = append(group.Conditions, db.Condition{
				Field:    fields[i],
				Operator: ops[i],
				Value:    vals[i],
			})
		}
	}
	rule.Groups = []db.ConditionGroup{group}
}

func parseActions(r *http.Request, rule *db.Rule) {
	rule.Actions = nil
	r.ParseForm()
	types := r.Form["action_type"]
	values := r.Form["action_value"]
	for i := range types {
		v := ""
		if i < len(values) {
			v = values[i]
		}
		rule.Actions = append(rule.Actions, db.Action{Type: types[i], Value: v})
	}
}
