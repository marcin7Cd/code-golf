package routes

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
)

func scores(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var holeID, langID string

	showDuplicates := strings.HasSuffix(r.URL.Path, "/all")

	if len(ps) == 1 {
		param := ps[0].Value

		if _, ok := holeByID[param]; ok {
			holeID = param
		} else if _, ok = langByID[param]; ok {
			langID = param
		} else {
			print404(w, r)
			return
		}
	} else if len(ps) == 2 {
		holeID = ps[0].Value
		langID = ps[1].Value

		if _, ok := holeByID[holeID]; !ok {
			print404(w, r)
			return
		}

		if langID == "all" {
			langID = ""
		} else if _, ok := langByID[langID]; !ok {
			print404(w, r)
			return
		}
	}

	userID := printHeader(w, r, 200)

	w.Write([]byte(
		"<script defer src=" + scoresJsPath +
			"></script><main id=scores><select id=hole><option value>All Holes",
	))

	for _, hole := range holes {
		w.Write([]byte("<option "))
		if holeID == hole.ID {
			w.Write([]byte("selected "))
		}
		w.Write([]byte("value=" + hole.ID + ">" + hole.Name))
	}

	w.Write([]byte("</select><select id=lang><option value>All Langs"))

	for _, lang := range langs {
		w.Write([]byte("<option "))
		if langID == lang.ID {
			w.Write([]byte("selected "))
		}
		w.Write([]byte("value=" + lang.ID + ">" + lang.Name))
	}

	w.Write([]byte("</select>"))

	if holeID != "" && langID == "" {
		w.Write([]byte("<label><input type=checkbox"))
		if showDuplicates {
			w.Write([]byte(" checked"))
		}
		w.Write([]byte(">Allow multiple entries per player</label>"))
	}

	w.Write([]byte("<table class=scores>"))

	var concat, distinct, table, where string

	if holeID != "" {
		where += " AND hole = '" + holeID + "'"
		concat = "' class=', lang, '>', TO_CHAR(strokes, 'FM99,999'), '<td>(', TO_CHAR(score, 'FM9,999'), ' point', CASE WHEN score"
	} else {
		concat = "'>', TO_CHAR(score, 'FM9,999'), '<td>(', count, ' hole', CASE WHEN count"
	}

	if showDuplicates {
		table = "scored_leaderboard"
	} else {
		distinct = "DISTINCT ON (hole, user_id)"
		table = "summed_leaderboard"
	}

	if langID != "" {
		where += " AND lang IN('" + langID + "')"
	}

	rows, err := db.Query(
		`WITH leaderboard AS (
		  SELECT `+distinct+`
		         hole,
		         submitted,
		         LENGTH(code) strokes,
		         user_id,
		         lang
		    FROM solutions
		   WHERE NOT failing`+where+`
		ORDER BY hole, user_id, LENGTH(code), submitted
		), scored_leaderboard AS (
		  SELECT hole,
		         ROUND(
		             (
		                 COUNT(*) OVER (PARTITION BY hole)
		                 -
		                 RANK() OVER (PARTITION BY hole ORDER BY strokes)
		                 +
		                 1
		             )
		             *
		             (
		                 100.0
		                 /
		                 COUNT(*) OVER (PARTITION BY hole)
		             )
		         ) score,
		         strokes,
		         submitted,
		         user_id,
		         lang
		    FROM leaderboard
		), summed_leaderboard AS (
		  SELECT user_id,
		         SUM(strokes)   strokes,
		         SUM(score)     score,
		         COUNT(*),
		         MAX(submitted) submitted,
		         STRING_AGG(CONCAT(lang), '') lang
		    FROM scored_leaderboard
		GROUP BY user_id
		) SELECT CONCAT(
		             '<tr',
		             CASE WHEN user_id = $1 THEN ' class=me' END,
		             '><td>',
		             TO_CHAR(
		                 RANK() OVER (ORDER BY score DESC, strokes),
		                 'FM999"<sup>"th"</sup>"'
		             ),
		             '<td><img src="//avatars.githubusercontent.com/',
		             login,
		             '?s=26"><a href="/users/',
		             login,
		             '">',
		             login,
		             '</a><td',
		             `+concat+` > 1 THEN 's' END,
		             ')<td><time datetime=',
		             TO_CHAR(submitted, 'YYYY-MM-DD"T"HH24:MI:SS"Z>"FMDD Mon'),
		             '</time>'
		         )
		    FROM `+table+`
		    JOIN users on user_id = id
		ORDER BY score DESC, strokes, submitted`,
		userID,
	)

	if err != nil {
		panic(err)
	}

	defer rows.Close()

	for rows.Next() {
		var row sql.RawBytes

		if err := rows.Scan(&row); err != nil {
			panic(err)
		}

		w.Write(row)
	}

	if err := rows.Err(); err != nil {
		panic(err)
	}
}
