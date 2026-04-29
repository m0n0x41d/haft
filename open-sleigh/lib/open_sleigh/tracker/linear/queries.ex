defmodule OpenSleigh.Tracker.Linear.Queries do
  @moduledoc """
  Pure GraphQL documents and variable builders for Linear.

  This module is intentionally transport-free. `OpenSleigh.Tracker.Linear`
  owns request execution and response normalisation; this module only
  carries the query shape.
  """

  @issue_fields """
  id
  identifier
  title
  description
  state {
    name
  }
  branchName
  url
  project {
    slugId
    name
  }
  createdAt
  updatedAt
  """

  @list_active_query """
  query OpenSleighLinearListActive($projectSlug: String!, $stateNames: [String!]!, $first: Int!, $after: String) {
    issues(filter: {project: {slugId: {eq: $projectSlug}}, state: {name: {in: $stateNames}}}, first: $first, after: $after) {
      nodes {
        #{@issue_fields}
      }
      pageInfo {
        hasNextPage
        endCursor
      }
    }
  }
  """

  @get_issue_query """
  query OpenSleighLinearGetIssue($issueId: String!) {
    issue(id: $issueId) {
      #{@issue_fields}
    }
  }
  """

  @state_id_query """
  query OpenSleighLinearResolveStateId($issueId: String!, $stateName: String!) {
    issue(id: $issueId) {
      team {
        states(filter: {name: {eq: $stateName}}, first: 1) {
          nodes {
            id
          }
        }
      }
    }
  }
  """

  @transition_mutation """
  mutation OpenSleighLinearTransition($issueId: String!, $stateId: String!) {
    issueUpdate(id: $issueId, input: {stateId: $stateId}) {
      success
    }
  }
  """

  @post_comment_mutation """
  mutation OpenSleighLinearPostComment($issueId: String!, $body: String!) {
    commentCreate(input: {issueId: $issueId, body: $body}) {
      success
    }
  }
  """

  @list_comments_query """
  query OpenSleighLinearListComments($issueId: String!, $first: Int!) {
    issue(id: $issueId) {
      comments(first: $first) {
        nodes {
          id
          body
          createdAt
          url
          user {
            email
            name
          }
        }
      }
    }
  }
  """

  @doc "Build the active-issue polling query."
  @spec list_active(String.t(), [String.t()], pos_integer(), String.t() | nil) ::
          {String.t(), map()}
  def list_active(project_slug, state_names, first, after_cursor)
      when is_binary(project_slug) and is_list(state_names) and is_integer(first) do
    variables =
      %{
        "projectSlug" => project_slug,
        "stateNames" => state_names,
        "first" => first,
        "after" => after_cursor
      }

    {@list_active_query, variables}
  end

  @doc "Build the single issue fetch query."
  @spec get_issue(String.t()) :: {String.t(), map()}
  def get_issue(issue_id) when is_binary(issue_id) do
    {@get_issue_query, %{"issueId" => issue_id}}
  end

  @doc "Build the workflow-state id lookup query."
  @spec state_id(String.t(), String.t()) :: {String.t(), map()}
  def state_id(issue_id, state_name) when is_binary(issue_id) and is_binary(state_name) do
    {@state_id_query, %{"issueId" => issue_id, "stateName" => state_name}}
  end

  @doc "Build the issue state transition mutation."
  @spec transition(String.t(), String.t()) :: {String.t(), map()}
  def transition(issue_id, state_id) when is_binary(issue_id) and is_binary(state_id) do
    {@transition_mutation, %{"issueId" => issue_id, "stateId" => state_id}}
  end

  @doc "Build the issue comment mutation."
  @spec post_comment(String.t(), String.t()) :: {String.t(), map()}
  def post_comment(issue_id, body) when is_binary(issue_id) and is_binary(body) do
    {@post_comment_mutation, %{"issueId" => issue_id, "body" => body}}
  end

  @doc "Build the issue comments query."
  @spec list_comments(String.t(), pos_integer()) :: {String.t(), map()}
  def list_comments(issue_id, first) when is_binary(issue_id) and is_integer(first) do
    {@list_comments_query, %{"issueId" => issue_id, "first" => first}}
  end
end
