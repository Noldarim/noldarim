import type { PipelineDraft } from "./types";

export type PipelineTemplate = {
  id: string;
  name: string;
  description: string;
  draft: PipelineDraft;
};

// Synced from examples in /examples/pipelines.
export const pipelineTemplates: PipelineTemplate[] = [
  {
    id: "simple-test",
    name: "Simple Test Pipeline v1.0.0",
    description: "A simple 2-step pipeline for testing",
    draft: {
      name: "Simple Test Pipeline v1.0.0",
      variables: {},
      steps: [
        {
          id: "step1",
          name: "Create file",
          prompt: "Create a file called hello4.txt with the content 'Hello from step 1'"
        },
        {
          id: "step2",
          name: "Append to file",
          prompt: "Append a new line 'Hello from step 2' to the hello4.txt file"
        }
      ]
    }
  },
  {
    id: "bug-fix",
    name: "Bug Fix",
    description: "Investigate and fix a bug with regression tests",
    draft: {
      name: "Bug Fix",
      variables: {
        bug_description: "describe the bug here"
      },
      steps: [
        {
          id: "investigate",
          name: "Investigate bug",
          prompt: "Investigate this bug: {{.bug_description}}\n\nFind the root cause and identify the files that need to be modified.\nDo not make changes yet - just analyze and understand the issue."
        },
        {
          id: "fix",
          name: "Implement fix",
          prompt: "Now implement the fix for the bug you just investigated.\nMake minimal, focused changes to address the root cause."
        },
        {
          id: "regression-test",
          name: "Add regression test",
          prompt: "Add a regression test that would have caught this bug.\nThe test should fail without your fix and pass with it."
        }
      ]
    }
  },
  {
    id: "feature-implementation",
    name: "Feature Implementation",
    description: "Implement a new feature with tests and documentation",
    draft: {
      name: "Feature Implementation",
      variables: {
        feature_name: "example-feature",
        language: "go"
      },
      steps: [
        {
          id: "implement",
          name: "Implement feature",
          prompt: "Implement the {{.feature_name}} feature.\nFollow the existing code patterns and conventions in this {{.language}} codebase.\nMake sure the implementation is clean and well-structured."
        },
        {
          id: "test",
          name: "Write tests",
          prompt: "Write comprehensive unit tests for the {{.feature_name}} feature you just implemented.\nCover edge cases and error scenarios.\nFollow the existing test patterns in the codebase."
        },
        {
          id: "docs",
          name: "Update documentation",
          prompt: "Update the relevant documentation to describe the {{.feature_name}} feature.\nInclude usage examples if appropriate."
        }
      ]
    }
  },
  {
    id: "refactor",
    name: "Code Refactor",
    description: "Refactor code in multiple passes",
    draft: {
      name: "Code Refactor",
      variables: {
        target: "specify what to refactor"
      },
      steps: [
        {
          id: "analyze",
          name: "Analyze current code",
          prompt: "Analyze the code related to: {{.target}}\n\nIdentify:\n- Code smells and areas for improvement\n- Potential abstractions that could be extracted\n- Duplicated logic that could be consolidated\n\nDo not make changes yet."
        },
        {
          id: "refactor",
          name: "Refactor code",
          prompt: "Based on your analysis, refactor the code.\n\nFocus on:\n- Improving readability and maintainability\n- Extracting reusable abstractions where appropriate\n- Removing duplication\n\nMake sure all existing tests still pass."
        },
        {
          id: "cleanup",
          name: "Final cleanup",
          prompt: "Do a final cleanup pass:\n- Remove any unused imports or variables\n- Ensure consistent formatting\n- Add comments only where the code isn't self-explanatory"
        }
      ]
    }
  },
  {
    id: "plan-spec-test-implement",
    name: "Plan Spec Test Write Run tests",
    description: "Implement new thing with many sensible steps",
    draft: {
      name: "Plan Spec Test Write Run tests",
      variables: {
        task_name: "Example task",
        language: "go"
      },
      steps: [
        {
          id: "Plan",
          name: "Create implementation plan",
          prompt: "Create a detailed implementation plan for {{.task_name}}.\nAnalyze the existing {{.language}} codebase structure and identify where changes need to be made.\nOutline the approach, key components, and any dependencies."
        },
        {
          id: "Spec",
          name: "Write specifications",
          prompt: "Read the previous step output: docs/ai_history/runs/{{.RunID}}/step-Plan.md\n\nBased on that plan, write detailed specifications for {{.task_name}}.\nDefine the interfaces, data structures, and expected behavior.\nInclude input/output contracts and error handling requirements."
        },
        {
          id: "Test",
          name: "Write tests based on specification",
          prompt: "Read the previous step outputs:\n- Plan: docs/ai_history/runs/{{.RunID}}/step-Plan.md\n- Spec: docs/ai_history/runs/{{.RunID}}/step-Spec.md\n\nBased on the specification, write comprehensive tests for {{.task_name}}.\nCover edge cases and error scenarios.\nFollow the existing test patterns in the {{.language}} codebase.\nThe tests should initially fail since the implementation doesn't exist yet."
        },
        {
          id: "Write",
          name: "Implement code to pass the tests",
          prompt: "Read the previous step outputs:\n- Spec: docs/ai_history/runs/{{.RunID}}/step-Spec.md\n- Test: docs/ai_history/runs/{{.RunID}}/step-Test.md\n\nImplement {{.task_name}} according to the specification.\nFollow the existing code patterns and conventions in this {{.language}} codebase.\nMake sure the implementation passes all the tests written in the Test step."
        },
        {
          id: "Run_tests",
          name: "Run tests and verify implementation",
          prompt: "Run the test suite for {{.task_name}} and report the results.\nIf any tests fail, analyze the failures and fix the implementation.\nEnsure all tests pass before completing this step."
        }
      ]
    }
  }
];
