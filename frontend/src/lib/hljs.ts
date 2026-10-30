import hljs from "highlight.js/lib/core"
import go from "highlight.js/lib/languages/go"
import typescript from "highlight.js/lib/languages/typescript"
import javascript from "highlight.js/lib/languages/javascript"
import python from "highlight.js/lib/languages/python"
import rust from "highlight.js/lib/languages/rust"
import java from "highlight.js/lib/languages/java"
import cpp from "highlight.js/lib/languages/cpp"
import c from "highlight.js/lib/languages/c"
import csharp from "highlight.js/lib/languages/csharp"
import php from "highlight.js/lib/languages/php"
import ruby from "highlight.js/lib/languages/ruby"
import kotlin from "highlight.js/lib/languages/kotlin"
import swift from "highlight.js/lib/languages/swift"
import dart from "highlight.js/lib/languages/dart"
import scala from "highlight.js/lib/languages/scala"
import bash from "highlight.js/lib/languages/bash"
import powershell from "highlight.js/lib/languages/powershell"
import sql from "highlight.js/lib/languages/sql"
import yaml from "highlight.js/lib/languages/yaml"
import json from "highlight.js/lib/languages/json"
import markdown from "highlight.js/lib/languages/markdown"
import xml from "highlight.js/lib/languages/xml"
import css from "highlight.js/lib/languages/css"
import scss from "highlight.js/lib/languages/scss"
import dockerfile from "highlight.js/lib/languages/dockerfile"
import makefile from "highlight.js/lib/languages/makefile"
import ini from "highlight.js/lib/languages/ini"
import graphql from "highlight.js/lib/languages/graphql"
import protobuf from "highlight.js/lib/languages/protobuf"
import lua from "highlight.js/lib/languages/lua"
import perl from "highlight.js/lib/languages/perl"
import r from "highlight.js/lib/languages/r"
import elixir from "highlight.js/lib/languages/elixir"
import haskell from "highlight.js/lib/languages/haskell"

const languages: Array<[string, unknown]> = [
  ["go", go],
  ["typescript", typescript],
  ["javascript", javascript],
  ["python", python],
  ["rust", rust],
  ["java", java],
  ["cpp", cpp],
  ["c", c],
  ["csharp", csharp],
  ["php", php],
  ["ruby", ruby],
  ["kotlin", kotlin],
  ["swift", swift],
  ["dart", dart],
  ["scala", scala],
  ["bash", bash],
  ["powershell", powershell],
  ["sql", sql],
  ["yaml", yaml],
  ["json", json],
  ["markdown", markdown],
  ["xml", xml],
  ["css", css],
  ["scss", scss],
  ["dockerfile", dockerfile],
  ["makefile", makefile],
  ["ini", ini],
  ["graphql", graphql],
  ["protobuf", protobuf],
  ["lua", lua],
  ["perl", perl],
  ["r", r],
  ["elixir", elixir],
  ["haskell", haskell],
]

for (const [name, language] of languages) {
  hljs.registerLanguage(name, language as never)
}

/** Loaded on demand by the code viewer; keeps grammars out of the entry bundle. */
export function ensureHljsLanguages(): void {
  // Registration happens as a side effect of this module being evaluated.
}
