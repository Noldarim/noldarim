# Graph View UX

- Fresh run (no Noldarim runs data)
-> Graph should show last 4 git commits (with current head at the top)

- With Noldarim runs data, graph view should be similar to git commits DAG with view enriched by the Noldarim run data (Piplines and steps that lead to new commits)
- Importantly the main view, should be a view of the whole commit tree, to which upon creation of new pipeline run, new branch is added which shows live execution
- The tree should support multiple async branch executions (as backend already should support this <- this claim must be double checked, and limits of current concurrency verfied).

- When user click on the pipeline either still running one or finished the user should be able to see the "detailed" pipelien view with steps nodes, and step details.

- It should be possible to specify any of the tree node (which represnt commit) as a new base_commit for a new pipeline to run. (currently clicked node should be marked as selected commit
to which new pipleline run should be appliable).

# data representation

## nodes

- all nodes should represent "new idempotent state of knowledge/data/code" created by applying "patch" to the previous node. In other words, nodes should represent static state of the commit.
(When we add that functioanlity stuff like current deployment url, current live logs, etc should be stuff visbile in the node. For now even just representing this node as a "commit" is good enoughj).
-

## Edges

- Edges should represent transitions/patches between states. Edges model the "pipeline running", and it's on the edges that the data of pipeline execution should "live".
- In the default graph view edges should cleary illustrate whether the pipeline is running, completed, failed or cancelled (by cancel operation triggered by user)
- Clicing on edges should display the "detailed" view of the pipeline/step (eg. agent output, tools used etc).
- When clicking on edge representing step it should be possible to see the step config that is currently exeecuted/was executed.
- When seeing the config of execution it should be possible to adjust that config, and request "new pipeline run with this step config changed"
- this should create a fork in the graph from the node from which the edge was originating, and create a new branch for the rest of remaing steps.
-> This behavior should be achived by calling the same pipeline on the backend api but with modified step in the pipeline template which should cause proper reuse by
the backend server of already created steps and commits, and only generate new commit for modified step.

## Tests

All above UX flows should be tested via unit tests of the nodes/edges and graphs, to make sure their interfaces work properly
