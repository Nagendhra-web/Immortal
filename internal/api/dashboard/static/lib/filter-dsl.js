// filter-dsl.js — micro-query parser for table filters.
// Grammar:
//   expr  := term (( "and"|"or" ) term)*
//   term  := field op value
//   op    := = | != | > | < | >= | <= | ~ (regex/substring)
// Quoted values allowed with '...' or "...".

export function parseFilter(input) {
  const tokens = tokenize(input);
  if (tokens.length === 0) return () => true;
  const ast = parseExpr(tokens, { i: 0 });
  return (row) => evalAst(ast, row);
}

function tokenize(input) {
  const re = /\s*(?:("(?:\\.|[^"])*"|'(?:\\.|[^'])*'|[a-zA-Z_][\w.-]*|>=|<=|!=|[=<>~]|\d+(?:\.\d+)?|\S))/y;
  const out = []; let m;
  re.lastIndex = 0;
  while ((m = re.exec(input))) out.push(m[1]);
  return out;
}

function parseExpr(toks, st) {
  let left = parseTerm(toks, st);
  while (toks[st.i] && /^(and|or)$/i.test(toks[st.i])) {
    const op = toks[st.i++].toLowerCase();
    const right = parseTerm(toks, st);
    left = { op, left, right };
  }
  return left;
}

function parseTerm(toks, st) {
  const field = toks[st.i++];
  const op    = toks[st.i++] || "=";
  let raw     = toks[st.i++];
  if (raw && (raw[0] === '"' || raw[0] === "'")) raw = raw.slice(1, -1);
  return { field, op, value: raw };
}

function evalAst(ast, row) {
  if (ast.op === "and") return evalAst(ast.left, row) && evalAst(ast.right, row);
  if (ast.op === "or")  return evalAst(ast.left, row) || evalAst(ast.right, row);
  const cell = resolve(row, ast.field);
  return compare(cell, ast.op, ast.value);
}

function resolve(row, path) {
  if (!row || !path) return undefined;
  return path.split(".").reduce((o, k) => (o == null ? o : o[k]), row);
}

function compare(cell, op, value) {
  if (cell === undefined || cell === null) return op === "!=";
  const nc = Number(cell), nv = Number(value);
  const num = !Number.isNaN(nc) && !Number.isNaN(nv);
  switch (op) {
    case "=":  return num ? nc === nv : String(cell) === String(value);
    case "!=": return num ? nc !== nv : String(cell) !== String(value);
    case ">":  return num && nc > nv;
    case "<":  return num && nc < nv;
    case ">=": return num && nc >= nv;
    case "<=": return num && nc <= nv;
    case "~":  return new RegExp(value, "i").test(String(cell));
    default:   return false;
  }
}
