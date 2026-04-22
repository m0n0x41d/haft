%{
  configs: [
    %{
      name: "default",
      files: %{
        included: ["lib/", "test/"],
        excluded: ["_build/", "deps/"]
      },
      plugins: [],
      requires: [],
      strict: true,
      parse_timeout: 5000,
      color: true,
      checks: %{
        enabled: [
          # Consistency
          {Credo.Check.Consistency.ExceptionNames, []},
          {Credo.Check.Consistency.LineEndings, []},
          {Credo.Check.Consistency.ParameterPatternMatching, []},
          {Credo.Check.Consistency.SpaceAroundOperators, []},
          {Credo.Check.Consistency.SpaceInParentheses, []},
          {Credo.Check.Consistency.TabsOrSpaces, []},

          # Design
          {Credo.Check.Design.AliasUsage, [priority: :low, if_nested_deeper_than: 2]},
          {Credo.Check.Design.TagTODO, [exit_status: 0]},
          {Credo.Check.Design.TagFIXME, [exit_status: 2]},

          # Readability — enforce specs on public funs
          {Credo.Check.Readability.ModuleDoc, []},
          {Credo.Check.Readability.Specs, [include_defp: false]},
          {Credo.Check.Readability.StrictModuleLayout, []},
          {Credo.Check.Readability.ModuleAttributeNames, []},
          {Credo.Check.Readability.ModuleNames, []},
          {Credo.Check.Readability.VariableNames, []},

          # Refactoring — keep CC low
          {Credo.Check.Refactor.CyclomaticComplexity, [max_complexity: 5]},
          {Credo.Check.Refactor.FunctionArity, [max_arity: 6]},
          {Credo.Check.Refactor.LongQuoteBlocks, []},
          {Credo.Check.Refactor.Nesting, [max_nesting: 2]},

          # Warnings
          {Credo.Check.Warning.BoolOperationOnSameValues, []},
          {Credo.Check.Warning.ExpensiveEmptyEnumCheck, []},
          {Credo.Check.Warning.IExPry, []},
          {Credo.Check.Warning.IoInspect, []},
          {Credo.Check.Warning.UnusedEnumOperation, []},
          {Credo.Check.Warning.UnusedFileOperation, []},
          {Credo.Check.Warning.UnusedKeywordOperation, []},
          {Credo.Check.Warning.UnusedListOperation, []},
          {Credo.Check.Warning.UnusedPathOperation, []},
          {Credo.Check.Warning.UnusedRegexOperation, []},
          {Credo.Check.Warning.UnusedStringOperation, []},
          {Credo.Check.Warning.UnusedTupleOperation, []}
        ],
        # Custom Open-Sleigh rules (to be implemented as proper Credo
        # plugins in L5; tracked as TODO markers for now).
        # TODO: OpenSleigh.Credo.NoDirectStructLiteral    — backstops PR1–PR10
        # TODO: OpenSleigh.Credo.NoObservationToHaftProxy — backstops OB5
        # TODO: OpenSleigh.Credo.NoDirectFilesystemIO     — backstops CL11
        # TODO: OpenSleigh.Credo.ImmutableCompiledConfig  — backstops CF8
        # TODO: OpenSleigh.Credo.NoDirectStructLiteral    — backstops PR1–PR10
        # TODO: OpenSleigh.Credo.HumanRoleOwnership       — backstops UP2
        # TODO: OpenSleigh.Credo.NoTokenCountInEvidence   — backstops TA1
        disabled: []
      }
    }
  ]
}
