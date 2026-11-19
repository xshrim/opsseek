<script lang="ts">
  import {
    AppWindow,
    AlertTriangle,
    ArrowLeft,
    ArrowRight,
    CheckCircle2,
    ClipboardCheck,
    ChevronDown,
    ChevronUp,
    Clock3,
    Link2,
    Map,
    Network,
    Pencil,
    Plus,
    RefreshCw,
    Search,
    Server,
    Sparkles,
    StickyNote,
    Trash2,
    X
  } from 'lucide-svelte';
  import {
    api,
    type Application,
    type Project,
    type ProjectTopology,
    type ProjectWorkspace,
    type InspectionPolicy,
    type InspectionRun,
    type Resource,
    type Team,
    type User
  } from '../../lib/api';
  import { errorMessage as formatErrorMessage } from '../../lib/errorMessage';
  import EntityBrandIcon from '../../components/EntityBrandIcon.svelte';
  import ResourceBrandIcon from '../../components/ResourceBrandIcon.svelte';
  import StatusBadge from '../../components/StatusBadge.svelte';
  import ExpandableTable from '../../components/ExpandableTable.svelte';
  import ReadonlyTable from '../../components/ReadonlyTable.svelte';
  import type { ReadonlyTableColumn } from '../../components/ReadonlyTable.svelte';
  import SearchInput from '../../components/SearchInput.svelte';
  import Combobox from '../../components/Combobox.svelte';
  import ScopeChip from '../../components/ScopeChip.svelte';
  import ScopeLevelBadge from '../../components/ScopeLevelBadge.svelte';
  import DeleteConfirmationDialog from '../../components/DeleteConfirmationDialog.svelte';
  import { labelTags } from '../../lib/labels';
  import { randomId } from '../../lib/randomId';
  import {
    resourceCategoryFor,
    resourceCategoryIcon,
    resourceDomainFor
  } from '../resource/resourceCatalog';
  import TagList from '../../components/TagList.svelte';
  import ApplicationWizard from './ApplicationWizard.svelte';
  import TopologyApplicationCreate from './TopologyApplicationCreate.svelte';
  import ProjectTopologyView from './ProjectTopology.svelte';
  import ProjectCreateForm from './ProjectCreateForm.svelte';
  import {
    runtimeLabel,
    matchesSearchFields,
    type ApplicationDraft,
    type ApplicationRuntimeKind,
    type ApplicationResourceCreationRequest,
    type ApplicationResourceCreationResult,
    type RuntimeResourceKind
  } from './projectTypes';
  import { onMount, tick } from 'svelte';

  export let teams: Team[] = [];
  export let allProjects: Project[] = [];
  export let visibleProjects: Project[] = [];
  export let availableResources: Resource[] = [];
  export let selectedScopeId = '';
  export let selectedProjectId = '';
  export let allTeamsSelected = false;
  export let busy = false;
  export let onSelectTeam: (team: Team) => void;
  export let onSelectAllTeams: () => void = () => {};
  export let onSelectProject: (project: Project) => void;
  export let onOpenTeamDialog: () => void;
  export let onProjectCreated: (project: Project) => void;
  export let onProjectUpdated: (project: Project) => void = () => {};
  export let onProjectDeleted: (project: Project) => void = () => {};
  export let canManageProject: (
    project: Project,
    action: 'update' | 'delete'
  ) => boolean = () => false;
  export let onNotice: (message: string) => void;
  export let onError: (message: string) => void;
  export let onOpenDiagnosis: (project: Project) => void;
  export let onOpenApplicationDiagnosis: (
    project: Project,
    application: Application
  ) => void = () => {};
  export let onOpenResource: (resource: Resource) => void = () => {};
  export let onNavigateToProjectApplication: (
    project: Project,
    applicationId: string
  ) => void = () => {};
  export let onResourceCreated: (resource: Resource) => void = () => {};
  export let onRequestResourceCreate: (
    request: ApplicationResourceCreationRequest
  ) => void = () => {};
  export let resourceCreationResult: ApplicationResourceCreationResult | null =
    null;
  export let resourceCreationCancelledId = '';
  export let projectAreaTab: 'applications' | 'topology' = 'applications';
  export let focusApplicationId = '';

  let workspace: ProjectWorkspace | null = null;
  let projectTopology: ProjectTopology | null = null;
  let inspectionPolicies: InspectionPolicy[] = [];
  let inspectionRuns: InspectionRun[] = [];
  let inspectionLoading = false;
  let inspectionReadFailed = false;
  let latestInspectionRuns = new globalThis.Map<string, InspectionRun>();
  let loadingTopology = false;
  let topologyMode: 'browse' | 'edit' = 'browse';
  let topologyResourceRequestId = '';
  type TopologyComponentIntent = {
    kind: 'application' | 'resource' | 'annotation';
    id?: string;
    resourceKind?: string;
    runtimeKind?: string;
    position?: { x: number; y: number };
  };
  type RelationshipApplicationRow = {
    application: Application;
    relation: Application['relations'][number] | null;
    annotation: ProjectTopology['nodes'][number] | null;
    projectName: string;
    applicationName: string;
  };
  let topologyComponentIntent: TopologyComponentIntent | null = null;
  let topologyApplicationId = '';
  let topologyResourceId = '';
  let topologyAnnotationName = '';
  let topologyAnnotationDescription = '';
  let topologyAnnotationProjectName = '';
  let topologyConfigBusy = false;
  let workspaceProjectId = '';
  let loadingWorkspace = false;
  let collapsedTeams = new Set<string>();
  let initializedTeamKey = '';
  let teamFilterElement: HTMLElement;
  let teamFilterMeasureElement: HTMLElement;
  let teamFilterExpanded = false;
  let visibleTeamCount = 0;
  let teamFilterHasOverflow = false;
  let measuredTeamFilterKey = '';
  let projectSearch = '';
  let workspaceSearch = '';
  let workspaceSearchFields: Record<string, string> = {};
  let relationTab: 'resources' | 'applications' = 'resources';
  let alertPanelTab: 'alerts' | 'inspections' = 'alerts';
  let applicationWizardTab: 'basic' | 'bindings' | 'applications' = 'basic';
  let projectCreateOpen = false;
  let editingProject: Project | null = null;
  let applicationWizardOpen = false;
  let topologyApplicationCreateOpen = false;
  let topologyEditApplication: Application | null = null;
  let editingApplication: Application | null = null;
  let deletingProject: Project | null = null;
  let deletingApplications: Application[] = [];
  let selectedApplicationIds: string[] = [];
  let inspectedApplicationID = '';
  let inspectedResourceIDs: string[] | null = null;
  let relationshipApplicationRows: RelationshipApplicationRow[] = [];
  let handledFocusApplicationId = '';
  let users: User[] = [];

  const applicationColumns: ReadonlyTableColumn[] = [
    { key: 'application', label: '应用', width: '18%' },
    { key: 'runtime', label: '运行时', width: '12%' },
    { key: 'listening_address', label: '监听地址', width: '14%' },
    { key: 'instances', label: '实例', width: '9%' },
    { key: 'resources', label: '资源', width: '8%' },
    { key: 'labels', label: '标签', width: '16%' },
    { key: 'status', label: '状态', width: '9%' },
    {
      key: 'actions',
      label: '操作',
      width: '10%',
      className: 'application-actions-column'
    }
  ];
  const applicationInstanceColumns: ReadonlyTableColumn[] = [
    { key: 'name', label: '实例', width: '22%' },
    { key: 'runtime', label: '运行时', width: '14%' },
    { key: 'resource', label: '运行资源', width: '24%' },
    { key: 'selector', label: '定位条件', width: '28%' },
    { key: 'status', label: '状态', width: '12%' }
  ];
  const workspaceSearchOptions = [
    { key: 'name', label: '应用名称', placeholder: '输入应用名称' },
    { key: 'code', label: '应用标识', placeholder: '输入应用标识' },
    { key: 'runtime', label: '运行时', placeholder: '输入运行时类型' },
    { key: 'status', label: '状态', placeholder: '输入应用状态' },
    { key: 'label', label: '标签', placeholder: '输入标签或 key=value' }
  ];

  onMount(() => {
    const resizeObserver = new ResizeObserver(() => void measureTeamFilter());
    if (teamFilterElement) resizeObserver.observe(teamFilterElement);
    void api
      .users()
      .then((result) => (users = result))
      .catch(() => (users = []));
    return () => resizeObserver.disconnect();
  });
  $: ownerOptions = users
    .filter((user) => user.status === 'active')
    .map((user) => ({
      value: user.id,
      label: user.display_name || user.username,
      description: user.display_name ? user.username : user.email || user.phone
    }));

  $: selectedProject =
    visibleProjects.find((project) => project.id === selectedProjectId) ??
    allProjects.find((project) => project.id === selectedProjectId) ??
    null;
  $: canUpdateSelectedProject = Boolean(
    selectedProject && canManageProject(selectedProject, 'update')
  );
  $: canDeleteSelectedProject = Boolean(
    selectedProject && canManageProject(selectedProject, 'delete')
  );
  $: topologyMode = canUpdateSelectedProject ? 'edit' : 'browse';
  $: selectedApplications =
    workspace?.applications.filter((application) =>
      selectedApplicationIds.includes(application.id)
    ) ?? [];
  $: if (
    focusApplicationId &&
    focusApplicationId !== handledFocusApplicationId &&
    workspace?.applications.some((item) => item.id === focusApplicationId)
  ) {
    handledFocusApplicationId = focusApplicationId;
    const application = workspace!.applications.find(
      (item) => item.id === focusApplicationId
    )!;
    workspaceSearch = application.name;
    selectedApplicationIds = [focusApplicationId];
    inspectedApplicationID = focusApplicationId;
    inspectedResourceIDs = null;
  }
  $: relationshipApplication =
    selectedApplicationIds.length === 1
      ? selectedApplications[0] ?? null
      : selectedApplicationIds.length > 1
        ? null
        : workspace?.applications.find(
            (application) => application.id === inspectedApplicationID
          ) ?? null;
  $: workspaceSearchPattern = workspaceSearchOptions
    .map((field) => field.key)
    .join('|');
  $: workspaceFreeSearch = workspaceSearch
    .replace(
      new RegExp(`(?:^|\\s)(?:${workspaceSearchPattern}):(?:"[^"]*"|'[^']*'|\\S*)`, 'g'),
      ' '
    )
    .trim()
    .toLocaleLowerCase();
  $: visibleWorkspaceApplications = (workspace?.applications ?? []).filter(
    (application) => {
      const searchable = {
        name: application.name,
        code: application.code,
        runtime: `${application.runtime_kind} ${runtimeLabel(application.runtime_kind as ApplicationRuntimeKind)}`,
        status: statusLabel(application.status),
        label: Object.entries(application.labels ?? {})
          .map(([key, value]) => `${key}=${value}`)
          .join(' ')
      };
      return matchesSearchFields(
        searchable,
        workspaceFreeSearch,
        workspaceSearchFields
      );
    }
  );
  $: inspectedResourceIDSet = inspectedResourceIDs
    ? new Set(inspectedResourceIDs)
    : null;
  $: visibleWorkspaceResources = (workspace?.resources ?? []).filter(
    (resource) =>
      !inspectedResourceIDSet || inspectedResourceIDSet.has(resource.id)
  );
  $: relationshipResourceRows = (() => {
    const applications = relationshipApplication
      ? [relationshipApplication]
      : workspace?.applications ?? [];
    const rows = applications.flatMap((application) =>
      applicationResourceIDs(application).flatMap((resourceId) => {
        const resource = (workspace?.resources ?? []).find(
          (item) => item.id === resourceId
        );
        return resource ? [{ application, resource }] : [];
      })
    );
    if (relationshipApplication) return rows;
    const linkedIDs = new Set(rows.map((row) => row.resource.id));
    return [
      ...rows,
      ...(workspace?.resources ?? [])
        .filter((resource) => !linkedIDs.has(resource.id))
        .map((resource) => ({ application: null, resource }))
    ];
  })();
  $: relationshipApplicationRows = (() => {
    const applications = relationshipApplication
      ? [relationshipApplication]
      : workspace?.applications ?? [];
    return applications.flatMap((application) => [
      ...(application.relations ?? []).map((relation) => {
        const targetNode = projectTopology?.nodes.find(
          (node) => node.id === relation.target_application_id
        );
        return {
          application,
          relation,
          annotation: null,
          projectName:
            targetNode?.project_name ??
            allProjects.find((project) => project.id === targetNode?.project_id)
              ?.name ??
            (targetNode?.project_id === selectedProjectId
              ? selectedProject?.name ?? ''
              : ''),
          applicationName:
            relation.target_application_name ?? relation.target_application_id
        };
      }),
      ...(projectTopology?.edges ?? []).flatMap((edge) => {
        if (edge.source_id !== application.id || edge.relation !== 'depends_on')
          return [];
        const annotation = projectTopology?.nodes.find(
          (node) =>
            node.id === edge.target_id &&
            node.kind === 'annotation' &&
            node.metadata?.dependency_source_application_id === application.id
        );
        if (!annotation) return [];
        return [{
          application,
          relation: null,
          annotation,
          projectName: String(annotation.metadata?.dependency_project_name ?? ''),
          applicationName: String(annotation.metadata?.description ?? annotation.name)
        }];
      })
    ]);
  })();
  $: visibleWorkspaceAlerts = (workspace?.alerts ?? []).filter(
    (alert) =>
      !inspectedResourceIDSet ||
      inspectedResourceIDSet.has(alert.target_resource_id)
  );
  $: {
    const latest = new globalThis.Map<string, InspectionRun>();
    for (const run of inspectionRuns) {
      const current = latest.get(run.policy_id);
      if (
        !current ||
        Date.parse(run.window_start) > Date.parse(current.window_start)
      )
        latest.set(run.policy_id, run);
    }
    latestInspectionRuns = latest;
  }
  $: topologyResourceOptions =
    topologyComponentIntent?.kind === 'resource'
      ? availableResources.filter(
          (resource) =>
            resourceCategoryFor(resource) ===
            topologyComponentIntent?.resourceKind
        )
      : availableResources;
  $: topologyResourceScopeTypes = Object.fromEntries(
    (workspace?.resources ?? []).map((resource) => [
      resource.id,
      resource.scope_type
    ])
  );
  $: applicationResourceScopeTypes = Object.fromEntries(
    availableResources.map((resource) => [
      resource.scope_id,
      topologyResourceScopeTypes[resource.id] ??
        (allProjects.some((project) => project.scope.id === resource.scope_id)
          ? 'project'
          : teams.some((team) => team.scope.id === resource.scope_id)
            ? 'team'
            : teams.some((team) => team.scope.parent_id === resource.scope_id)
              ? 'platform'
              : 'scope')
    ])
  );
  $: applicationAlertCounts = Object.fromEntries(
    (workspace?.applications ?? []).map((application) => {
      const resourceIDs = new Set(applicationResourceIDs(application));
      return [
        application.id,
        (workspace?.alerts ?? []).filter((alert) =>
          resourceIDs.has(alert.target_resource_id)
        ).length
      ];
    })
  );
  $: normalizedProjectSearch = projectSearch.trim().toLocaleLowerCase();
  $: filteredProjects = normalizedProjectSearch
    ? visibleProjects.filter((project) =>
        [project.name, project.code].some((value) =>
          value.toLocaleLowerCase().includes(normalizedProjectSearch)
        )
      )
    : visibleProjects;
  $: groupedProjects = teams.map((team) => ({
    team,
    projects: filteredProjects.filter((project) => project.team_id === team.id)
  }));
  $: teamKey = teams.map((team) => team.id).join(',');
  $: teamFilterKey = `${teamKey}:${allProjects.map((project) => `${project.team_id}:${project.id}`).join(',')}:${teams.map((team) => team.name).join('|')}:${teamFilterExpanded}`;
  $: if (teamFilterKey !== measuredTeamFilterKey) {
    measuredTeamFilterKey = teamFilterKey;
    void measureTeamFilter();
  }
  $: if (teamKey !== initializedTeamKey) {
    initializedTeamKey = teamKey;
    collapsedTeams = new Set(teams.slice(3).map((team) => team.id));
  }
  $: if (selectedProjectId && selectedProjectId !== workspaceProjectId) {
    selectedApplicationIds = [];
    inspectedApplicationID = '';
    inspectedResourceIDs = null;
    void loadWorkspace(selectedProjectId);
  }
  $: if (!selectedProjectId) {
    workspace = null;
    projectTopology = null;
    workspaceProjectId = '';
    inspectionPolicies = [];
    inspectionRuns = [];
    inspectionLoading = false;
    inspectionReadFailed = false;
    selectedApplicationIds = [];
  }
  $: if (topologyMode === 'browse' || projectAreaTab !== 'topology')
    topologyComponentIntent = null;

  async function measureTeamFilter() {
    await tick();
    if (!teamFilterElement || !teamFilterMeasureElement) return;
    if (teamFilterExpanded) {
      visibleTeamCount = teams.length;
      return;
    }
    const allChip = teamFilterElement.querySelector<HTMLElement>('.scope-chip');
    const addButton =
      teamFilterElement.querySelector<HTMLElement>('.team-filter-add');
    const teamChips = Array.from(
      teamFilterMeasureElement.querySelectorAll<HTMLElement>('.scope-chip')
    );
    const moreProbe = teamFilterMeasureElement.querySelector<HTMLElement>(
      '.team-filter-more-probe'
    );
    if (!allChip || !addButton || !moreProbe) return;
    const gap =
      Number.parseFloat(getComputedStyle(teamFilterElement).columnGap) || 0;
    const fixedWidth =
      allChip.getBoundingClientRect().width +
      addButton.getBoundingClientRect().width;
    const availableWidth = teamFilterElement.clientWidth;
    for (let count = teams.length; count >= 0; count -= 1) {
      const hiddenCount = teams.length - count;
      const moreWidth = hiddenCount
        ? moreProbe.getBoundingClientRect().width
        : 0;
      const itemCount = count + 2 + (hiddenCount ? 1 : 0);
      const teamsWidth = teamChips
        .slice(0, count)
        .reduce((sum, chip) => sum + chip.getBoundingClientRect().width, 0);
      if (
        fixedWidth +
          teamsWidth +
          moreWidth +
          gap * Math.max(itemCount - 1, 0) <=
        availableWidth
      ) {
        visibleTeamCount = count;
        teamFilterHasOverflow = hiddenCount > 0;
        return;
      }
    }
    visibleTeamCount = 0;
    teamFilterHasOverflow = teams.length > 0;
  }

  function describeError(error: unknown, fallback: string) {
    return formatErrorMessage(error, fallback);
  }
  function teamName(id: string) {
    return teams.find((team) => team.id === id)?.name ?? '未分配团队';
  }
  function statusLabel(status: string) {
    return ['active', 'healthy', 'up', 'normal'].includes(status)
      ? '正常'
      : status === 'disabled'
        ? '已停用'
        : status === 'warning'
          ? '需关注'
          : '未知';
  }
  function inspectionRunLabel(status: string) {
    return status === 'succeeded'
      ? '成功'
      : status === 'failed'
        ? '失败'
        : status === 'running'
          ? '执行中'
          : status === 'queued'
            ? '排队中'
            : status === 'degraded'
              ? '部分成功'
              : '未知';
  }
  function inspectionRunTime(value: string) {
    return Number.isNaN(Date.parse(value))
      ? '时间未知'
      : new Date(value).toLocaleString();
  }
  function projectStatus(project: Project) {
    return project.status === 'active' && project.summary.alerts === 0
      ? '正常'
      : project.status === 'disabled'
        ? '已停用'
        : '异常';
  }
  function projectHealthTone(project: Project) {
    return projectStatus(project) === '正常'
      ? 'success'
      : project.status === 'disabled'
        ? 'disabled'
        : 'warning';
  }
  function projectUpdatedLabel(value: string) {
    const timestamp = Date.parse(value);
    if (!Number.isFinite(timestamp)) return '更新时间未知';
    const minutes = Math.floor(Math.max(Date.now() - timestamp, 0) / 60000);
    return minutes < 1
      ? '刚刚更新'
      : minutes < 60
        ? `${minutes} 分钟前更新`
        : `${Math.floor(minutes / 60)} 小时前更新`;
  }
  function projectTags(project: Project) {
    return labelTags(project.labels);
  }
  function toggleTeam(id: string) {
    const next = new Set(collapsedTeams);
    next.has(id) ? next.delete(id) : next.add(id);
    collapsedTeams = next;
  }
  function applicationInstanceRows(application: Application) {
    return application.instances.map((instance) => ({
      name: instance.name,
      runtime: runtimeLabel(
        instance.runtime_kind as
          'virtual_machine' | 'containerized' | 'cloud_native'
      ),
      resource: instance.target_resource_name || instance.target_resource_id,
      selector:
        Object.entries(instance.selector)
          .filter(([, value]) => typeof value === 'string' && value)
          .map(([key, value]) => key + '=' + value)
          .join(' · ') || '—',
      status: statusLabel(instance.status),
      tone: instance.status
    }));
  }

  function applicationRuntimeResourceNames(application: Application) {
    if (!application.instances.length) return '未知';
    return (
      [
        ...new Set(
          application.instances
            .map(
              (instance) =>
                instance.target_resource_name || instance.target_resource_id
            )
            .filter(Boolean)
        )
      ].join('、') || '—'
    );
  }

  function applicationResourceCount(application: Application) {
    return new Set(
      [
        ...application.instances.map((instance) => instance.target_resource_id),
        ...(application.dependencies ?? []).map(
          (dependency) => dependency.target_resource_id
        )
      ].filter(Boolean)
    ).size;
  }

  function applicationResourceIDs(application: Application) {
    return [
      ...new Set(
        [
          ...application.instances.map(
            (instance) => instance.target_resource_id
          ),
          ...(application.dependencies ?? []).map(
            (dependency) => dependency.target_resource_id
          )
        ].filter(Boolean)
      )
    ];
  }

  function inspectApplication(application: Application) {
    inspectedApplicationID = application.id;
    inspectedResourceIDs = applicationResourceIDs(application);
  }

  function inspectTopologySelection(selection: {
    node: import('../../lib/api').ProjectTopology['nodes'][number] | null;
    edge: import('../../lib/api').ProjectTopology['edges'][number] | null;
  }) {
    const appIDs = new Set<string>();
    const resourceIDs = new Set<string>();
    const addNode = (id: string) => {
      const node = projectTopology?.nodes.find((item) => item.id === id);
      if (node?.kind === 'application') appIDs.add(node.id);
      if (node?.kind === 'resource') resourceIDs.add(node.id);
    };
    if (selection.node) {
      addNode(selection.node.id);
      inspectedApplicationID =
        selection.node.kind === 'application' ? selection.node.id : '';
    } else if (selection.edge) {
      addNode(selection.edge.source_id);
      addNode(selection.edge.target_id);
      inspectedApplicationID = '';
    } else {
      inspectedApplicationID = '';
      inspectedResourceIDs = null;
      return;
    }
    for (const id of appIDs) {
      const application = workspace?.applications.find(
        (item) => item.id === id
      );
      if (application)
        applicationResourceIDs(application).forEach((resourceID) =>
          resourceIDs.add(resourceID)
        );
    }
    inspectedResourceIDs = [...resourceIDs];
  }

  function setProjectAreaTab(tab: 'applications' | 'topology') {
    projectAreaTab = tab;
    inspectedApplicationID = '';
    inspectedResourceIDs = null;
    if (tab === 'applications') topologyComponentIntent = null;
  }

  function scopeLevelLabel(scopeType: string) {
    return scopeType === 'platform'
      ? '平台'
      : scopeType === 'team'
        ? '团队'
        : '项目';
  }

  function ownerLabel(ids: string[]) {
    const names = ids.map(
      (id) => ownerOptions.find((option) => option.value === id)?.label || id
    );
    return names.length ? names.join('、') : '未配置';
  }
  async function loadWorkspace(id: string) {
    loadingWorkspace = true;
    loadingTopology = true;
    inspectionPolicies = [];
    inspectionRuns = [];
    inspectionLoading = true;
    inspectionReadFailed = false;
    try {
      const [nextWorkspace, nextTopology] = await Promise.all([
        api.projectWorkspace(id),
        api.projectTopology(id)
      ]);
      workspace = nextWorkspace;
      projectTopology = nextTopology;
      workspaceProjectId = id;
      const scopeId = selectedProject?.scope.id ?? selectedScopeId;
      const inspectionResults = await Promise.allSettled([
        api.inspectionPolicies(scopeId),
        api.inspectionRuns(scopeId)
      ]);
      if (selectedProjectId !== id) return;
      inspectionPolicies =
        inspectionResults[0].status === 'fulfilled'
          ? inspectionResults[0].value
          : [];
      inspectionRuns =
        inspectionResults[1].status === 'fulfilled'
          ? inspectionResults[1].value
          : [];
      inspectionReadFailed = inspectionResults.some(
        (result) => result.status === 'rejected'
      );
    } catch (error) {
      onError(describeError(error, '项目驾驶舱加载失败'));
    } finally {
      loadingWorkspace = false;
      loadingTopology = false;
      inspectionLoading = false;
    }
  }
  async function loadTopology() {
    if (!selectedProjectId) return;
    loadingTopology = true;
    try {
      projectTopology = await api.projectTopology(selectedProjectId);
      workspace = await api.projectWorkspace(selectedProjectId);
    } catch (error) {
      onError(describeError(error, '项目拓扑刷新失败'));
    } finally {
      loadingTopology = false;
    }
  }
  function requestTopologyResource(kind: string) {
    const id = randomId();
    topologyResourceRequestId = id;
    onRequestResourceCreate({
      id,
      kind,
      scopeId: selectedProject?.scope.id ?? selectedScopeId,
      coverageTarget: '.map-side-column'
    });
  }
  $: if (
    resourceCreationCancelledId &&
    resourceCreationCancelledId === topologyResourceRequestId
  ) {
    topologyResourceRequestId = '';
    topologyComponentIntent = null;
  }
  $: if (
    resourceCreationResult &&
    resourceCreationResult.requestId === topologyResourceRequestId
  ) {
    topologyResourceRequestId = '';
    void (async () => {
      try {
        await api.addProjectTopologyResource(
          selectedProjectId,
          resourceCreationResult.resource.id
        );
        if (topologyComponentIntent?.position)
          await api.setProjectTopologyPosition(
            selectedProjectId,
            resourceCreationResult.resource.id,
            topologyComponentIntent.position
          );
        topologyComponentIntent = null;
        await loadTopology();
      } catch (error) {
        onError(describeError(error, '资源已创建，但加入项目拓扑失败'));
      }
    })();
  }
  async function useExistingTopologyComponent(intent: TopologyComponentIntent) {
    if (!selectedProjectId || !intent.id) return;
    try {
      if (intent.kind === 'application') {
        const node = projectTopology?.nodes.find(
          (item) => item.kind === 'application' && item.id === intent.id
        );
        if (!node) throw new Error('应用不在当前项目拓扑中');
        if (intent.position)
          await api.setProjectTopologyPosition(
            selectedProjectId,
            node.id,
            intent.position
          );
        if (intent.position) await loadTopology();
      } else if (intent.kind === 'resource') {
        await api.addProjectTopologyResource(selectedProjectId, intent.id);
        if (intent.position)
          await api.setProjectTopologyPosition(
            selectedProjectId,
            intent.id,
            intent.position
          );
        await loadTopology();
      }
      onNotice('拓扑组件已添加');
    } catch (error) {
      onError(describeError(error, '添加拓扑组件失败'));
    }
  }
  function configureTopologyComponent(intent: TopologyComponentIntent) {
    if (intent.kind === 'application' && !intent.id) {
      topologyComponentIntent = intent;
      topologyApplicationCreateOpen = true;
      return;
    }
    if (intent.kind === 'resource' && !intent.id) {
      topologyComponentIntent = intent;
      requestTopologyResource(intent.resourceKind ?? 'Host');
      return;
    }
    if (intent.id) {
      void useExistingTopologyComponent(intent);
      return;
    }
    topologyComponentIntent = intent;
    topologyApplicationId =
      intent.kind === 'application' && intent.id
        ? intent.id
        : (workspace?.applications[0]?.id ?? '');
    topologyResourceId =
      intent.kind === 'resource' && intent.id
        ? intent.id
        : (availableResources.find(
            (resource) => resourceCategoryFor(resource) === intent.resourceKind
          )?.id ?? '');
    topologyAnnotationName = '';
    topologyAnnotationDescription = '';
    topologyAnnotationProjectName = '';
  }
  function cancelTopologyComponent() {
    if (topologyConfigBusy) return;
    topologyApplicationCreateOpen = false;
    topologyComponentIntent = null;
  }
  async function saveTopologyComponent() {
    const intent = topologyComponentIntent;
    if (!intent || !selectedProjectId || topologyConfigBusy) return;
    topologyConfigBusy = true;
    try {
      if (intent.kind === 'application') {
        const node = projectTopology?.nodes.find(
          (item) =>
            item.kind === 'application' && item.id === topologyApplicationId
        );
        if (!node) throw new Error('请选择当前项目中已存在的应用');
        topologyComponentIntent = null;
        if (intent.position)
          await api.setProjectTopologyPosition(
            selectedProjectId,
            node.id,
            intent.position
          );
        await loadTopology();
      } else if (intent.kind === 'resource') {
        if (!topologyResourceId) throw new Error('请选择资源');
        await api.addProjectTopologyResource(
          selectedProjectId,
          topologyResourceId
        );
        if (intent.position)
          await api.setProjectTopologyPosition(
            selectedProjectId,
            topologyResourceId,
            intent.position
          );
        topologyComponentIntent = null;
        await loadTopology();
      } else {
        const name = topologyAnnotationName.trim();
        if (!name) throw new Error('请填写实体标题');
        const node = await api.createProjectTopologyAnnotation(
          selectedProjectId,
          {
            name,
            metadata: {
              description: topologyAnnotationDescription.trim(),
              project_name: topologyAnnotationProjectName.trim()
            }
          }
        );
        if (intent.position)
          await api.setProjectTopologyPosition(
            selectedProjectId,
            node.id,
            intent.position
          );
        topologyComponentIntent = null;
        await loadTopology();
      }
      onNotice('拓扑组件已保存');
    } catch (error) {
      onError(describeError(error, '保存拓扑组件失败'));
    } finally {
      topologyConfigBusy = false;
    }
  }
  function applicationPayload(draft: ApplicationDraft) {
    return {
      name: draft.name,
      code: draft.code,
      description: draft.description,
      icon: draft.icon,
      labels: draft.labels,
      runtime_kind: draft.runtimeKind,
      external_uid: draft.externalUid,
      critical: draft.critical,
      technical_owner_ids: draft.technicalOwnerIds,
      operations_owner_ids: draft.operationsOwnerIds,
      dependencies: draft.dependencies ?? [],
      relations: draft.relations ?? [],
      instances: draft.instances.map((instance) => ({
        name: instance.name,
        runtime_kind: draft.runtimeKind,
        target_resource_id: instance.targetResourceId,
        selector: instance.selector,
        log_binding: instance.logBinding,
        status: instance.status
      }))
    };
  }
  async function saveProject(input: {
    teamId: string;
    name: string;
    code: string;
    icon: string;
    description: string;
    labels: Record<string, string>;
    importanceLevel: 'ITP1' | 'ITP2' | 'ITP3' | 'ITP4';
    technicalOwnerIds: string[];
    operationsOwnerIds: string[];
    applications: ApplicationDraft[];
  }) {
    if (editingProject) {
      const project = await api.updateProject(editingProject.id, {
        name: input.name,
        description: input.description,
        icon: input.icon,
        labels: input.labels,
        importance_level: input.importanceLevel,
        technical_owner_ids: input.technicalOwnerIds,
        operations_owner_ids: input.operationsOwnerIds
      });
      editingProject = null;
      onProjectUpdated(project);
      onNotice(`项目“${project.name}”已更新`);
      return;
    }
    const project = await api.createProject(input.teamId, {
      name: input.name,
      code: input.code,
      description: input.description,
      icon: input.icon,
      labels: input.labels,
      importance_level: input.importanceLevel,
      technical_owner_ids: input.technicalOwnerIds,
      operations_owner_ids: input.operationsOwnerIds,
      applications: input.applications.map(applicationPayload)
    });
    projectCreateOpen = false;
    onProjectCreated(project);
    onSelectProject(project);
    onNotice(`项目“${project.name}”已创建`);
  }
  async function saveApplications(drafts: ApplicationDraft[]) {
    if (!selectedProjectId) return;
    const addedFromTopology = topologyComponentIntent?.kind === 'application';
    const dropPosition = topologyComponentIntent?.position;
    const result = await api.createApplications(
      selectedProjectId,
      drafts.map(applicationPayload)
    );
    await Promise.all(
      result.items.map((application, index) =>
        syncApplicationExternalDependencies(
          application.id,
          drafts[index]?.externalDependencies ?? []
        )
      )
    );
    applicationWizardOpen = false;
    await loadWorkspace(selectedProjectId);
    if (addedFromTopology) {
      if (dropPosition)
        await Promise.all(
          result.items.map((application, index) =>
            api.setProjectTopologyPosition(selectedProjectId, application.id, {
              x: Math.min(1171, dropPosition.x + index * 24),
              y: Math.min(9960, dropPosition.y + index * 24)
            })
          )
        );
      topologyComponentIntent = null;
      await loadTopology();
    }
    onNotice(`${drafts.length} 个应用已新增`);
  }
  async function saveTopologyApplication(draft: ApplicationDraft) {
    if (!selectedProjectId || topologyComponentIntent?.kind !== 'application')
      return;
    const dropPosition = topologyComponentIntent.position;
    const application = await api.createApplication(
      selectedProjectId,
      applicationPayload(draft)
    );
    await syncApplicationExternalDependencies(
      application.id,
      draft.externalDependencies ?? []
    );
    topologyApplicationCreateOpen = false;
    if (dropPosition)
      await api.setProjectTopologyPosition(
        selectedProjectId,
        application.id,
        dropPosition
      );
    topologyComponentIntent = null;
    await loadWorkspace(selectedProjectId);
    await loadTopology();
    onNotice(`应用“${application.name}”已新增`);
  }
  async function saveTopologyApplicationEdit(draft: ApplicationDraft) {
    if (!topologyEditApplication) return;
    await updateApplication(topologyEditApplication, draft);
    topologyEditApplication = null;
    await loadTopology();
  }
  async function updateApplication(app: Application, draft: ApplicationDraft) {
    try {
      await api.updateApplication(selectedProjectId, app.id, {
        name: draft.name,
        code: draft.code,
        description: draft.description,
        icon: draft.icon,
        runtime_kind: draft.runtimeKind,
        external_uid: draft.externalUid,
        labels: draft.labels,
        critical: draft.critical,
        technical_owner_ids: draft.technicalOwnerIds,
        operations_owner_ids: draft.operationsOwnerIds,
        dependencies: draft.dependencies ?? [],
        relations: draft.relations ?? [],
        instances: applicationPayload(draft).instances
      });
      await syncApplicationExternalDependencies(
        app.id,
        draft.externalDependencies ?? []
      );
      editingApplication = null;
      await loadWorkspace(selectedProjectId);
      onNotice('应用已更新');
    } catch (error) {
      throw new Error(describeError(error, '更新应用失败'));
    }
  }

  async function syncApplicationExternalDependencies(
    applicationId: string,
    dependencies: Array<{ projectName: string; description: string }>
  ) {
    const existing = (projectTopology?.nodes ?? []).filter(
      (node) =>
        node.kind === 'annotation' &&
        node.metadata?.dependency_source_application_id === applicationId
    );
    const retained = new Set<string>();
    for (const dependency of dependencies) {
      const projectName = dependency.projectName.trim();
      const description = dependency.description.trim();
      const prior = existing.find(
        (node) =>
          !retained.has(node.id) &&
          node.name === description &&
          String(node.metadata?.dependency_project_name ?? '') === projectName
      );
      if (prior) {
        retained.add(prior.id);
        continue;
      }
      const node = await api.createProjectTopologyAnnotation(selectedProjectId, {
        name: description,
        metadata: {
          description,
          dependency_project_name: projectName,
          dependency_source_application_id: applicationId
        }
      });
      await api.createProjectTopologyEdge(selectedProjectId, {
        source_id: applicationId,
        target_id: node.id,
        relation: 'depends_on',
        metadata: { kind: 'external_dependency' }
      });
      retained.add(node.id);
    }
    await Promise.all(
      existing
        .filter((node) => !retained.has(node.id))
        .map((node) => api.deleteProjectTopologyAnnotation(selectedProjectId, node.id))
    );
  }

  function applicationUpdateBody(
    application: Application,
    relations = application.relations,
    dependencies = application.dependencies,
    instances = application.instances
  ) {
    return {
      name: application.name,
      code: application.code,
      description: application.description,
      icon: application.icon,
      runtime_kind: application.runtime_kind,
      external_uid: application.external_uid ?? '',
      labels: application.labels,
      critical: application.critical,
      technical_owner_ids: application.technical_owner_ids,
      operations_owner_ids: application.operations_owner_ids,
      dependencies: dependencies.map((dependency) => ({
        dependency_kind: dependency.dependency_kind,
        target_resource_id: dependency.target_resource_id,
        binding: dependency.binding,
        required: dependency.required
      })),
      relations: relations.map((relation) => ({
        target_application_id: relation.target_application_id,
        relation_kind: relation.relation_kind,
        status: relation.status
      })),
      instances: instances.map((instance) => ({
        name: instance.name,
        runtime_kind: instance.runtime_kind,
        target_resource_id: instance.target_resource_id,
        selector: instance.selector,
        log_binding: instance.log_binding,
        status: instance.status
      }))
    };
  }

  async function removeResourceAssociation(application: Application, resourceId: string) {
    const current = await api.application(application.project_id, application.id);
    await api.updateApplication(
      current.project_id,
      current.id,
      applicationUpdateBody(
        current,
        current.relations,
        current.dependencies.filter((dependency) => dependency.target_resource_id !== resourceId),
        current.instances.filter((instance) => instance.target_resource_id !== resourceId)
      )
    );
    await loadWorkspace(selectedProjectId);
  }

  async function removeApplicationAssociation(
    application: Application,
    relationId: string,
    annotationId = ''
  ) {
    if (annotationId) {
      await api.deleteProjectTopologyAnnotation(selectedProjectId, annotationId);
    } else {
      const current = await api.application(application.project_id, application.id);
      await api.updateApplication(
        current.project_id,
        current.id,
        applicationUpdateBody(
          current,
          current.relations.filter((relation) => relation.id !== relationId)
        )
      );
    }
    await loadWorkspace(selectedProjectId);
  }

  async function openApplicationEditor(
    application: Application,
    tab: 'basic' | 'bindings' | 'applications' = 'basic'
  ) {
    const projectId = selectedProjectId || application.project_id;
    applicationWizardTab = tab;
    try {
      editingApplication = await api.application(projectId, application.id);
      applicationWizardTab = tab;
    } catch (error) {
      onError(describeError(error, '读取应用详情失败'));
      // Keep the row data available as a fallback so the editor can still open.
      editingApplication = application;
    }
  }

  async function openTopologyApplicationEditor(application: Application) {
    const projectId = selectedProjectId || application.project_id;
    try {
      topologyEditApplication = await api.application(projectId, application.id);
    } catch (error) {
      onError(describeError(error, '读取应用详情失败'));
      topologyEditApplication =
        workspace?.applications.find((item) => item.id === application.id) ??
        application;
    }
  }
  async function confirmProjectDelete() {
    if (!deletingProject) return;
    const project = deletingProject;
    try {
      await api.deleteProject(project.id, project.name);
      deletingProject = null;
      onProjectDeleted(project);
      onSelectAllTeams();
      onNotice(
        `项目“${project.name}”及其应用和范围配置已删除；关联资源已保留至团队范围`
      );
    } catch (error) {
      onError(describeError(error, '删除项目失败'));
    }
  }
  async function confirmApplicationDelete() {
    if (!deletingApplications.length || !selectedProjectId) return;
    try {
      const externalDependencyNodes = (projectTopology?.nodes ?? []).filter(
        (node) =>
          node.kind === 'annotation' &&
          deletingApplications.some(
            (application) =>
              node.metadata?.dependency_source_application_id === application.id
          )
      );
      await Promise.all(
        externalDependencyNodes.map((node) =>
          api.deleteProjectTopologyAnnotation(selectedProjectId, node.id)
        )
      );
      await Promise.all(
        deletingApplications.map((application) =>
          api.deleteApplication(selectedProjectId, application.id)
        )
      );
      const deletedCount = deletingApplications.length;
      deletingApplications = [];
      selectedApplicationIds = [];
      await loadWorkspace(selectedProjectId);
      onNotice(String(deletedCount) + ' 个应用已删除，关联资源未受影响');
    } catch (error) {
      onError(describeError(error, '删除应用失败'));
    }
  }
</script>

<section class="resource-map-page">
  {#if !selectedProject}
    <header class="resource-map-header project-overview-header">
      <div class="project-page-heading">
        <span class="project-page-heading-icon" aria-hidden="true"
          ><Map size={19} /></span
        >
        <div>
          <h2><strong>项目全景图</strong><small>PROJECT OVERVIEW</small></h2>
          <p>按团队查看项目分布，进入项目查看运行状态与资源关系。</p>
        </div>
      </div>
      <div class="resource-map-actions">
        <SearchInput
          bind:value={projectSearch}
          className="project-overview-search"
          width="160px"
          placeholder="搜索项目名称或编号"
          ariaLabel="搜索项目名称或编号"
        /><button
          class="primary"
          type="button"
          on:click={() => (projectCreateOpen = true)}
          ><Plus size={15} />新增项目</button
        >
      </div>
    </header>
    <nav
      class:expanded={teamFilterExpanded}
      class="team-filter"
      aria-label="团队筛选"
      bind:this={teamFilterElement}
    >
      <ScopeChip
        label="全部团队"
        count={allProjects.length}
        active={allTeamsSelected}
        on:click={onSelectAllTeams}
      />{#each teamFilterExpanded ? teams : teams.slice(0, visibleTeamCount) as team}<ScopeChip
          label={team.name}
          count={allProjects.filter((project) => project.team_id === team.id)
            .length}
          active={selectedScopeId === team.scope.id}
          on:click={() => onSelectTeam(team)}
        />{/each}{#if teamFilterHasOverflow}<button
          class="team-filter-expand"
          type="button"
          aria-label={teamFilterExpanded
            ? '收起团队列表'
            : `展开其余 ${teams.length - visibleTeamCount} 个团队`}
          aria-expanded={teamFilterExpanded}
          data-tooltip={teamFilterExpanded
            ? '收起团队'
            : `展开其余 ${teams.length - visibleTeamCount} 个团队`}
          on:click={() => (teamFilterExpanded = !teamFilterExpanded)}
          >{#if teamFilterExpanded}<ChevronUp size={14} />{:else}<span
              >+{teams.length - visibleTeamCount}</span
            ><ChevronDown size={14} />{/if}</button
        >{/if}<button
        class="secondary team-filter-add"
        type="button"
        disabled={busy}
        on:click={onOpenTeamDialog}><Plus size={14} />新增团队</button
      >
      <div
        class="team-filter-measure"
        aria-hidden="true"
        inert
        bind:this={teamFilterMeasureElement}
      >
        {#each teams as team}<ScopeChip
            label={team.name}
            count={allProjects.filter((project) => project.team_id === team.id)
              .length}
          />{/each}<button
          class="team-filter-expand team-filter-more-probe"
          type="button"
          ><span>+{teams.length}</span><ChevronDown size={14} /></button
        >
      </div>
    </nav>
    {#if normalizedProjectSearch && filteredProjects.length === 0}<div
        class="map-empty project-search-empty"
      >
        <Search size={28} />
        <h2>没有匹配项目</h2>
        <p>未找到名称或编号包含“{projectSearch.trim()}”的项目。</p>
        <button
          class="secondary"
          type="button"
          on:click={() => (projectSearch = '')}>清除搜索</button
        >
      </div>{:else}<div class="team-project-groups">
        {#each groupedProjects as group}{#if group.projects.length || (!selectedScopeId && !normalizedProjectSearch)}{@const teamSelected =
              selectedScopeId === group.team.scope.id}{@const teamCollapsed =
              !normalizedProjectSearch &&
              collapsedTeams.has(group.team.id) &&
              !teamSelected}
            <section class="team-project-group">
              <button
                class="group-heading group-heading-toggle"
                type="button"
                aria-expanded={!teamCollapsed}
                on:click={() => toggleTeam(group.team.id)}
                ><span class="group-title"
                  ><EntityBrandIcon
                    kind="Team"
                    fallback={group.team.icon || 'lucide:Building'}
                    size={16}
                    className="team-mark"
                  /><span
                    ><h2>{group.team.name}</h2>
                    <small>{group.team.description || '团队项目'}</small></span
                  ></span
                ><span class="group-heading-end"
                  ><span class="group-count"
                    >{group.projects.length} 个项目</span
                  ><span
                    class:rotate={teamCollapsed}
                    class="team-collapse-icon"
                    aria-hidden="true"><ChevronDown size={16} /></span
                  ></span
                ></button
              >{#if !teamCollapsed}<div class="project-card-grid">
                  {#each group.projects as project}<article
                      class="project-map-card"
                      role="group"
                      aria-label={project.name}
                    >
                      <button
                        class="project-card-open"
                        type="button"
                        aria-label={`打开项目 ${project.name}`}
                        on:click={() => onSelectProject(project)}
                      ></button>
                      <div class="project-card-top">
                        <EntityBrandIcon
                          kind="Project"
                          fallback={project.icon || 'lucide:FolderKanban'}
                          size={20}
                          className="project-card-icon"
                        />
                        <div class="project-card-identity">
                          <div class="project-card-title">
                            <strong>{project.name}</strong>
                            <span
                              class={`project-importance-tag ${project.importance_level.toLowerCase()}`}
                              >{project.importance_level}</span
                            >
                          </div>
                          <small>{project.code}</small>
                        </div>
                        <span class="project-card-arrow"
                          ><ArrowRight size={17} /></span
                        >
                      </div>
                      <div class="project-card-middle">
                        <div class="project-card-primary">
                          <div class="project-card-application-health">
                            <strong
                              >{project.summary.applications_healthy} / {project
                                .summary.applications}</strong
                            >
                            <small>正常应用</small>
                          </div>
                          <TagList
                            tags={projectTags(project)}
                            className="project-card-tags"
                            ariaLabel="项目标签"
                            distinguishKeyOnly={false}
                            fit
                            wrap={false}
                          />
                        </div>
                        <div
                          class="project-card-secondary-metrics"
                          aria-label="项目运行统计"
                        >
                          <span
                            ><small>关联资源</small><strong
                              >{project.summary.resources}</strong
                            ></span
                          >
                          <span
                            ><small>关联</small><strong
                              >{project.summary.dependencies}</strong
                            ></span
                          >
                          <span
                            ><small>项目告警</small><strong
                              class:warning={project.summary.alerts > 0}
                              >{project.summary.alerts}</strong
                            ></span
                          >
                        </div>
                      </div>
                      <div class="project-card-footer">
                        <span class="project-card-health-summary">
                          <StatusBadge
                            className="status-label"
                            tone={projectHealthTone(project)}
                            label={projectStatus(project)}
                          />
                          <span
                            class="project-health-separator"
                            aria-hidden="true">·</span
                          >
                          <span class="project-card-updated"
                            ><Clock3 size={12} />{projectUpdatedLabel(
                              project.updated_at
                            )}</span
                          >
                        </span>
                        <span class="project-card-actions">
                          {#if canManageProject(project, 'update')}<button
                              class="icon-button edit-action"
                              type="button"
                              data-tooltip="编辑项目"
                              aria-label={`编辑项目 ${project.name}`}
                              on:click={() => (editingProject = project)}
                              ><Pencil size={14} aria-hidden="true" /></button
                            >{/if}
                          {#if canManageProject(project, 'delete')}<button
                              class="icon-button danger-action delete-action"
                              type="button"
                              data-tooltip="删除项目"
                              aria-label={`删除项目 ${project.name}`}
                              on:click={() => (deletingProject = project)}
                              ><Trash2 size={14} aria-hidden="true" /></button
                            >{/if}
                        </span>
                      </div>
                    </article>{:else}<div class="map-empty compact">
                      <Network size={20} /><span>该团队还没有项目</span><button
                        type="button"
                        class="text-button"
                        on:click={() => (projectCreateOpen = true)}
                        >新增项目</button
                      >
                    </div>{/each}
                </div>{/if}
            </section>{/if}{/each}
      </div>{/if}
  {:else}
    <header class="workspace-header resource-map-header">
      <div class="workspace-title">
        <div class="project-heading">
          <button
            class="project-heading-back"
            type="button"
            data-tooltip="返回项目全景图"
            aria-label="返回项目全景图"
            on:click={onSelectAllTeams}><ArrowLeft size={18} /></button
          >
          <div>
            <h1>
              <strong>项目驾驶舱</strong><small class="project-heading-english"
                >PROJECT COCKPIT</small
              >
            </h1>
            <p class="project-heading-context">
              {teamName(selectedProject.team_id)} / {selectedProject.name} · {selectedProject.code}
            </p>
          </div>
        </div>
      </div>
      <div class="workspace-actions">
        <button
          class="secondary"
          type="button"
          disabled={loadingWorkspace}
          data-tooltip="刷新项目驾驶舱"
          aria-label="刷新项目驾驶舱"
          on:click={() =>
            selectedProject && void loadWorkspace(selectedProject.id)}
          ><RefreshCw size={15} />刷新</button
        >
        {#if canDeleteSelectedProject}<button
            class="danger-button workspace-action"
            type="button"
            on:click={() => (deletingProject = selectedProject)}
            ><Trash2 size={15} />删除项目</button
          >{/if}<button
          class="primary workspace-action"
          type="button"
          on:click={() => (projectCreateOpen = true)}
          ><Plus size={15} />新增项目</button
        ><button
          class="diagnose"
          type="button"
          data-tooltip="进入 AI 诊断"
          on:click={() => onOpenDiagnosis(selectedProject)}
          ><Sparkles size={15} />AI 诊断</button
        >
      </div>
    </header>
    <div class="workspace-stats">
            {#each [['应用', workspace?.summary.applications ?? 0, '应用和服务'], ['实例', workspace?.summary.instances ?? 0, '运行中的实例'], ['资源', workspace?.summary.resources ?? 0, '当前项目可见范围'], ['关联', workspace?.summary.dependencies ?? 0, '暂未配置'], ['告警', workspace?.summary.alerts ?? 0, '待处理信号']] as stat}<div
          class="workspace-stat"
        >
          <span>{stat[0]}</span><strong>{stat[1]}</strong><small
            >{stat[2]}</small
          >
        </div>{/each}
    </div>
    <div class="resource-map-columns">
      <section class="panel project-applications-area map-main-column">
        <div class="project-area-toolbar">
          <div class="project-area-tabs" role="tablist" aria-label="项目视图">
            <button
              class:active={projectAreaTab === 'applications'}
              role="tab"
              aria-selected={projectAreaTab === 'applications'}
              on:click={() => setProjectAreaTab('applications')}
            >
              <AppWindow size={16} />应用
            </button>
            <button
              class:active={projectAreaTab === 'topology'}
              role="tab"
              aria-selected={projectAreaTab === 'topology'}
              on:click={() => setProjectAreaTab('topology')}
            >
              <Network size={16} />拓扑
            </button>
          </div>
          <div class="project-area-search-wrap">
            <SearchInput
              bind:value={workspaceSearch}
              fieldSearchable
              fieldOptions={workspaceSearchOptions}
              width="100%"
              height="34px"
              ariaLabel="搜索应用"
              placeholder="搜索应用或输入 name:、code:"
              on:fields={(event) => (workspaceSearchFields = event.detail)}
            />
          </div>
          <div class="section-heading-actions project-area-actions">
            <span class="muted">{visibleWorkspaceApplications.length} 个应用</span
            >
            {#if projectAreaTab === 'applications'}
              <button
                class="secondary"
                type="button"
                disabled={loadingWorkspace}
                data-tooltip="刷新应用"
                aria-label="刷新应用"
                on:click={() =>
                  selectedProject && void loadWorkspace(selectedProject.id)}
                ><RefreshCw size={15} />刷新</button
              >{#if canDeleteSelectedProject}<button
                  class="danger-button workspace-action"
                  type="button"
                  disabled={!selectedApplications.length}
                  data-tooltip={selectedApplications.length
                    ? `删除所选 ${selectedApplications.length} 个应用`
                    : '请先选择应用'}
                  on:click={() => (deletingApplications = selectedApplications)}
                  ><Trash2 size={15} />删除</button
                >{/if}{#if canUpdateSelectedProject}<button
                  class="primary workspace-action"
                  type="button"
                  on:click={() => (applicationWizardOpen = true)}
                  ><Plus size={15} />新增</button
                >{/if}
            {:else}
              <button
                class="secondary"
                type="button"
                disabled={loadingTopology}
                data-tooltip="刷新拓扑"
                aria-label="刷新拓扑"
                on:click={() => void loadTopology()}
                ><RefreshCw size={15} />刷新</button
              >
            {/if}
          </div>
        </div>
        {#if projectAreaTab === 'applications'}
          <div class="project-applications-panel">
            {#if loadingWorkspace}<div class="map-loading">
                正在加载项目驾驶舱...
              </div>
            {:else if visibleWorkspaceApplications.length}
              <ExpandableTable
                columns={applicationColumns}
                rows={visibleWorkspaceApplications}
                rowKey={(application) => application.id}
                expandable
                selectable
                selectedIds={selectedApplicationIds}
                onSelectionChange={(ids) => (selectedApplicationIds = ids)}
                onRowClick={inspectApplication}
                rowClass={(row) =>
                  row.id === inspectedApplicationID
                    ? 'inspected-application-row selected'
                    : ''}
                emptyText="当前项目暂无匹配应用"
              >
                <svelte:fragment slot="cell" let:row let:column>
                  {#if column.key === 'application'}
                    <span class="application-table-name"
                      ><AppWindow
                        class="project-application-node-icon"
                        size={17}
                      /><span
                        ><strong>{row.name}</strong><small>{row.code}</small
                        ></span
                      >{#if applicationAlertCounts[row.id]}<span
                          class="application-alert-indicator"
                          data-tooltip={`${applicationAlertCounts[row.id]} 条关联告警`}
                          aria-label={`${applicationAlertCounts[row.id]} 条关联告警`}
                          ><AlertTriangle size={13} />{applicationAlertCounts[
                            row.id
                          ]}</span
                        >{/if}</span
                    >
                  {:else if column.key === 'runtime'}
                    <span
                      class="application-table-value"
                      data-tooltip={applicationRuntimeResourceNames(row)}
                      ><strong>{applicationRuntimeResourceNames(row)}</strong
                      ><small
                        >{row.instances.length
                          ? row.runtime_kind === 'virtual_machine'
                            ? '虚拟机'
                            : row.runtime_kind === 'containerized'
                              ? '容器化'
                              : '云原生'
                          : '未绑定运行时'}</small
                      ></span
                    >
                  {:else if column.key === 'listening_address'}<span
                      class="application-table-value"
                      data-tooltip={row.listening_address || '未发现监听地址'}
                      ><strong>{row.listening_address || '—'}</strong></span
                    >
                  {:else if column.key === 'instances'}<span
                      class="application-table-count"
                      >{row.instances.length} 个实例</span
                    >
                  {:else if column.key === 'resources'}<span
                      class="application-table-count"
                      >{applicationResourceCount(row)} 个资源</span
                    >
                  {:else if column.key === 'labels'}<TagList
                      className="application-table-tags"
                      tags={labelTags(row.labels)}
                      fit
                      wrap={false}
                      colorScheme="independent"
                      maxWidth="130px"
                      emptyText="—"
                    />
                  {:else if column.key === 'status'}<StatusBadge
                      className="status-label"
                      tone={row.status}
                      label={statusLabel(row.status)}
                    />
                  {:else if column.key === 'actions'}
                    <span class="application-table-actions">
                      <button
                        class="icon-button edit-action"
                        type="button"
                        data-tooltip="AI 诊断"
                        aria-label="AI 诊断"
                        on:click|stopPropagation={() =>
                          onOpenApplicationDiagnosis(selectedProject, row)}
                        ><Sparkles size={14} /></button
                      >
                      {#if canUpdateSelectedProject}<button
                          class="icon-button edit-action"
                          type="button"
                          data-tooltip="编辑应用"
                          aria-label="编辑应用"
                          on:click|stopPropagation={() =>
                            void openApplicationEditor(row)}
                          ><Pencil size={15} /></button
                        >{/if}
                      {#if canDeleteSelectedProject}<button
                          class="icon-button danger-action delete-action"
                          type="button"
                          data-tooltip="删除应用"
                          aria-label={`删除应用 ${row.name}`}
                          on:click|stopPropagation={() =>
                            (deletingApplications = [row])}
                          ><Trash2 size={14} /></button
                        >{/if}
                    </span>
                  {/if}
                </svelte:fragment>
                <svelte:fragment slot="details" let:row>
                  <div class="application-detail-grid">
                    <div class="application-detail-fields">
                      <div>
                        <small>应用名称</small><strong>{row.name}</strong>
                      </div>
                      <div>
                        <small>应用标识</small><strong>{row.code}</strong>
                      </div>
                      <div>
                        <small>运行时</small><strong
                          >{row.instances.length
                            ? runtimeLabel(row.runtime_kind)
                            : '未知'}</strong
                        >
                      </div>
                      <div>
                        <small>监听地址</small><strong
                          >{row.listening_address || '—'}</strong
                        >
                      </div>
                      <div>
                        <small>关键</small><strong
                          >{row.critical ? '是' : '否'}</strong
                        >
                      </div>
                      <div>
                        <small>技术负责人</small><strong
                          >{ownerLabel(row.technical_owner_ids)}</strong
                        >
                      </div>
                      <div>
                        <small>运维负责人</small><strong
                          >{ownerLabel(row.operations_owner_ids)}</strong
                        >
                      </div>
                      <div>
                        <small>标签</small><TagList
                          tags={labelTags(row.labels)}
                        />
                      </div>
                      <div>
                        <small>描述</small><strong
                          >{row.description || '—'}</strong
                        >
                      </div>
                    </div>
                    <ReadonlyTable
                      columns={applicationInstanceColumns}
                      rows={applicationInstanceRows(row)}
                      emptyText="未绑定运行实例"
                    >
                      <svelte:fragment
                        slot="cell"
                        let:row={instance}
                        let:column
                      >
                        {#if column.key === 'status'}<StatusBadge
                            className="status-label"
                            tone={instance.tone}
                            label={instance.status}
                          />{:else}{instance[column.key]}{/if}
                      </svelte:fragment>
                    </ReadonlyTable>
                  </div>
                </svelte:fragment>
              </ExpandableTable>
            {:else}<div class="map-empty">
                <Network size={28} />
                <h3>还没有应用</h3>
                <p>使用应用向导新增 Host、Docker 或 Kubernetes 应用。</p>
                {#if canUpdateSelectedProject}<button
                    class="primary"
                    type="button"
                    on:click={() => (applicationWizardOpen = true)}
                    ><Plus size={15} />新增应用</button
                  >{/if}
              </div>{/if}
          </div>
        {:else}
          <div class="project-topology-panel">
            <ProjectTopologyView
              project={selectedProject!}
              topology={projectTopology}
              applications={workspace?.applications ?? []}
              resources={availableResources}
              resourceScopeTypes={topologyResourceScopeTypes}
              searchQuery={workspaceFreeSearch}
              searchFields={workspaceSearchFields}
              loading={loadingTopology}
              mode={topologyMode}
              configurationOpen={Boolean(topologyComponentIntent)}
              pendingComponent={topologyComponentIntent}
              {applicationAlertCounts}
              onSelectionChange={inspectTopologySelection}
              onRefresh={loadTopology}
              onConfigureNode={configureTopologyComponent}
              onEditApplication={(application) =>
                void openTopologyApplicationEditor(application)}
              {onNotice}
              {onError}
            />
          </div>
        {/if}
      </section>
      <aside
        class="map-side-column"
        class:topology-configuring={Boolean(topologyComponentIntent)}
      >
        {#if projectAreaTab === 'topology' && topologyMode === 'edit' && topologyEditApplication}
          <div class="topology-application-create-region">
            <TopologyApplicationCreate
              resources={availableResources}
              {users}
              runtimeKind={topologyEditApplication.runtime_kind as ApplicationRuntimeKind}
              initialApplication={topologyEditApplication}
              onSave={saveTopologyApplicationEdit}
              onCancel={() => (topologyEditApplication = null)}
            />
          </div>
        {:else if projectAreaTab === 'topology' && topologyMode === 'edit' && topologyApplicationCreateOpen && topologyComponentIntent?.kind === 'application'}
          <div class="topology-application-create-region">
            <TopologyApplicationCreate
              resources={availableResources}
              {users}
              runtimeKind={topologyComponentIntent.runtimeKind as ApplicationRuntimeKind}
              onSave={saveTopologyApplication}
              onCancel={() => {
                topologyApplicationCreateOpen = false;
                topologyComponentIntent = null;
              }}
            />
          </div>
        {:else}
          <section class="panel map-side-panel relationship-panel">
            <div class="relationship-heading">
              <div class="project-area-tabs relationship-tabs" role="tablist" aria-label="关联依赖类型">
                <button
                  class:active={relationTab === 'resources'}
                  type="button"
                  role="tab"
                  aria-selected={relationTab === 'resources'}
                  on:click={() => (relationTab = 'resources')}
                  ><Server size={16} />资源</button
                ><button
                  class:active={relationTab === 'applications'}
                  type="button"
                  role="tab"
                  aria-selected={relationTab === 'applications'}
                  on:click={() => (relationTab = 'applications')}
                  ><Link2 size={16} />依赖</button
                >
              </div>
              <span class="relationship-selected-name">
                {relationshipApplication?.name ?? ''}
              </span>
              {#if canUpdateSelectedProject}<button
                  class="icon-button"
                  type="button"
                  aria-label={relationTab === 'resources' ? '添加资源关联' : '添加应用依赖'}
                  data-tooltip={relationTab === 'resources' ? '添加资源关联' : '添加应用依赖'}
                  disabled={!relationshipApplication}
                  on:click={() =>
                    relationshipApplication &&
                    void openApplicationEditor(
                      relationshipApplication,
                      relationTab === 'resources' ? 'bindings' : 'applications'
                    )}
                  ><Plus size={15} /></button
                >{/if}
            </div>
            {#if relationTab === 'resources'}
              {#each relationshipResourceRows as row, index (`${row.application?.id ?? 'unlinked'}:${row.resource.id}:${index}`)}
                <div class="relationship-row resource-association-row">
                  <div class="relationship-row-app">
                    {#if row.application}<AppWindow
                        class="project-application-node-icon"
                        size={15}
                      /><button
                        class="text-link"
                        type="button"
                        on:click={() =>
                          onNavigateToProjectApplication(
                            selectedProject!,
                            row.application!.id
                          )}>{row.application.name}</button
                      >{:else}<span class="muted">—</span>{/if}
                  </div>
                  <span class="relationship-resource-name">
                    <button
                      class="text-link"
                      class:runtime-resource-link={resourceDomainFor(row.resource.kind) === '运行时'}
                      class:regular-resource-link={resourceDomainFor(row.resource.kind) !== '运行时'}
                      type="button"
                      on:click={() => {
                        const resource = availableResources.find(
                          (item) => item.id === row.resource.id
                        );
                        if (resource) onOpenResource(resource);
                      }}>{row.resource.name}</button
                    ><ScopeLevelBadge
                      type={row.resource.scope_type as 'platform' | 'team' | 'project'}
                      label={scopeLevelLabel(row.resource.scope_type)}
                    />
                  </span>
                  <span class="relationship-resource-type">
                    <ResourceBrandIcon
                      resource={{ kind: row.resource.kind }}
                      size={15}
                    />
                    <span>{row.resource.kind}</span>
                  </span>
                  <span class="relationship-row-actions">
                    {#if row.application && canUpdateSelectedProject}<button
                        class="icon-button"
                        type="button"
                        aria-label="编辑资源关联"
                        data-tooltip="编辑资源关联"
                        on:click={() =>
                          void openApplicationEditor(row.application!, 'bindings')}
                        ><Pencil size={13} /></button
                      >{/if}{#if row.application && canDeleteSelectedProject}<button
                        class="icon-button danger-action"
                        type="button"
                        aria-label="删除资源关联"
                        data-tooltip="删除资源关联"
                        on:click={() =>
                          void removeResourceAssociation(
                            row.application!,
                            row.resource.id
                          ).catch((error) =>
                            onError(describeError(error, '删除资源关联失败'))
                          )}
                        ><Trash2 size={13} /></button
                      >{/if}
                  </span>
                </div>
              {:else}<div class="detail-empty">暂无资源关联</div>{/each}
            {:else}
              {#each relationshipApplicationRows as row, index (`${row.application.id}:${row.relation?.id ?? row.annotation?.id ?? index}`)}
                <div class="relationship-row application-dependency-row">
                  <div class="relationship-row-app">
                    <AppWindow
                      class="project-application-node-icon"
                      size={15}
                    /><button
                      class="text-link"
                      type="button"
                      on:click={() =>
                        onNavigateToProjectApplication(
                          selectedProject!,
                          row.application.id
                        )}>{row.application.name}</button
                    >
                  </div>
                  <span class="relationship-row-target">
                    {#if row.projectName}<button
                        class="text-link dependency-target-link"
                        type="button"
                        on:click={() => {
                          const project = allProjects.find(
                            (item) => item.name === row.projectName
                          );
                          if (project) onSelectProject(project);
                        }}><EntityBrandIcon
                          kind="Project"
                          fallback="lucide:FolderKanban"
                          className="relationship-icon project-tone"
                          size={15}
                        />{row.projectName}</button
                      >{:else}<span class="muted">未指定项目</span>{/if}
                    <button
                      class="text-link dependency-target-link"
                      type="button"
                      on:click={() => {
                        const targetProject = allProjects.find(
                          (item) => item.name === row.projectName
                        ) ?? selectedProject;
                        if (targetProject)
                          onNavigateToProjectApplication(
                            targetProject,
                            row.relation?.target_application_id ?? row.applicationName
                          );
                      }}><AppWindow
                        class="project-application-node-icon"
                        size={15}
                      />{row.applicationName}</button
                    >
                  </span>
                  <span class="relationship-row-actions">
                    {#if canUpdateSelectedProject}<button
                      class="icon-button"
                      type="button"
                      aria-label="编辑应用依赖"
                      data-tooltip="编辑应用依赖"
                      on:click={() =>
                        void openApplicationEditor(row.application, 'applications')}
                      ><Pencil size={13} /></button
                    >{/if}{#if canDeleteSelectedProject}<button
                      class="icon-button danger-action"
                      type="button"
                      aria-label="删除应用依赖"
                      data-tooltip="删除应用依赖"
                      on:click={() =>
                        void removeApplicationAssociation(
                          row.application,
                          row.relation?.id ?? '',
                          row.annotation?.id ?? ''
                        ).catch((error) =>
                          onError(describeError(error, '删除应用依赖失败'))
                        )}
                      ><Trash2 size={13} /></button
                    >{/if}
                  </span>
                </div>
              {:else}<div class="detail-empty">暂无应用依赖</div>{/each}
            {/if}
          </section>
          <section class="panel map-side-panel alerts-panel">
            <div class="relationship-heading alert-inspection-heading">
              <div class="project-area-tabs relationship-tabs" role="tablist" aria-label="告警与巡检">
                <button
                  class:active={alertPanelTab === 'alerts'}
                  type="button"
                  role="tab"
                  aria-selected={alertPanelTab === 'alerts'}
                  on:click={() => (alertPanelTab = 'alerts')}
                  ><AlertTriangle size={16} />告警</button
                ><button
                  class:active={alertPanelTab === 'inspections'}
                  type="button"
                  role="tab"
                  aria-selected={alertPanelTab === 'inspections'}
                  on:click={() => (alertPanelTab = 'inspections')}
                  ><ClipboardCheck size={16} />巡检</button
                >
              </div>
            </div>
            {#if alertPanelTab === 'alerts'}
              {#each visibleWorkspaceAlerts as alert}<div class="alert-map-row">
                  <span class="severity {alert.severity}"></span><span
                    ><strong>{alert.title}</strong><small
                      >{statusLabel(alert.status)}</small
                    ></span
                  >
                </div>{:else}<div class="detail-empty success-empty">
                  <CheckCircle2 size={15} />当前没有项目告警
                </div>{/each}
            {:else if inspectionLoading}
              <div class="detail-empty">正在加载巡检任务…</div>
            {:else if inspectionReadFailed}
              <div class="detail-empty">巡检任务暂不可用</div>
            {:else}
              {#each inspectionPolicies as policy (policy.id)}
                {@const latestRun = latestInspectionRuns.get(policy.id)}
                <article class="inspection-map-row">
                  <div>
                    <strong>{policy.name}</strong>
                    <small class="inspection-map-frequency">频率：{policy.cron} · {policy.timezone}</small>
                    <small class="inspection-map-execution">
                      {#if latestRun}
                        最近执行：{inspectionRunLabel(latestRun.status)} · {inspectionRunTime(latestRun.window_start)}
                      {:else}
                        尚无执行记录
                      {/if}
                    </small>
                  </div>
                  <span class:disabled={policy.status !== 'active'} class="inspection-policy-status">
                    {policy.status === 'active' ? '启用' : '已停用'}
                  </span>
                </article>
              {:else}
                <div class="detail-empty">本项目尚未配置巡检任务</div>
              {/each}
            {/if}
          </section>
          {#if projectAreaTab === 'topology' && topologyMode === 'edit' && topologyComponentIntent && !applicationWizardOpen && !topologyApplicationCreateOpen && !topologyResourceRequestId}
            <div class="topology-config-overlay">
              <section class="topology-config-window" aria-label="配置拓扑组件">
                <header>
                  <span class="topology-config-icon"
                    >{#if topologyComponentIntent.kind === 'application'}<AppWindow
                        class="project-application-node-icon"
                        size={17}
                      />{:else if topologyComponentIntent.kind === 'resource'}<ResourceBrandIcon
                        resource={{
                          kind: topologyComponentIntent.resourceKind ?? 'Host'
                        }}
                        fallback={resourceCategoryIcon(
                          topologyComponentIntent.resourceKind ?? 'Host'
                        )}
                        size={17}
                      />{:else}<StickyNote size={17} />{/if}</span
                  >
                  <div>
                    <strong
                      >{topologyComponentIntent.kind === 'application'
                        ? '配置应用节点'
                        : topologyComponentIntent.kind === 'resource'
                          ? '配置资源节点'
                          : '配置实体'}</strong
                    ><small>确认组件内容后添加到项目拓扑</small>
                  </div>
                </header>
                <div class="topology-config-body">
                  {#if topologyComponentIntent.kind === 'application'}
                    <label
                      >应用<select bind:value={topologyApplicationId}
                        ><option value="">选择应用</option
                        >{#each workspace?.applications ?? [] as application (application.id)}<option
                            value={application.id}
                            >{application.name} · {application.code}</option
                          >{/each}</select
                      ></label
                    >
                    <button
                      class="secondary"
                      type="button"
                      on:click={() => (applicationWizardOpen = true)}
                      ><Plus size={14} />新增应用</button
                    >
                  {:else if topologyComponentIntent.kind === 'resource'}
                    <label
                      >资源<select bind:value={topologyResourceId}
                        ><option value="">选择资源</option
                        >{#each topologyResourceOptions as resource (resource.id)}<option
                            value={resource.id}
                            >{resource.name} · {resource.kind}</option
                          >{/each}</select
                      ></label
                    >
                    <button
                      class="secondary"
                      type="button"
                      on:click={() =>
                        requestTopologyResource(
                          topologyComponentIntent?.resourceKind || 'Host'
                        )}><Plus size={14} />新增资源</button
                    >
                  {:else}
                    <label
                      >名称<input
                        bind:value={topologyAnnotationName}
                        maxlength="200"
                        placeholder="例如：支付核心系统"
                      /></label
                    >
                    <label
                      >项目（可选）<input
                        bind:value={topologyAnnotationProjectName}
                        maxlength="200"
                        placeholder="例如：订单平台"
                      /></label
                    >
                    <label
                      >说明<textarea
                        bind:value={topologyAnnotationDescription}
                        rows="4"
                        placeholder="说明其系统边界、职责或连接方式"
                      ></textarea></label
                    >
                  {/if}
                </div>
                <footer>
                  <button
                    class="secondary"
                    type="button"
                    disabled={topologyConfigBusy}
                    on:click={cancelTopologyComponent}>取消</button
                  ><button
                    class="primary"
                    type="button"
                    disabled={topologyConfigBusy}
                    on:click={saveTopologyComponent}
                    ><CheckCircle2 size={14} />{topologyConfigBusy
                      ? '保存中'
                      : '确认并添加'}</button
                  >
                </footer>
              </section>
            </div>
          {/if}
        {/if}
      </aside>
    </div>
  {/if}
</section>

{#if projectCreateOpen || editingProject}<ProjectCreateForm
    {teams}
    resources={availableResources}
    resourceScopeTypes={applicationResourceScopeTypes}
    {users}
    initialProject={editingProject}
    defaultTeamId={teams.find((team) => team.scope.id === selectedScopeId)
      ?.id ??
      teams[0]?.id ??
      ''}
    onSave={saveProject}
    onCancel={() => {
      projectCreateOpen = false;
      editingProject = null;
    }}
  />{/if}
{#if applicationWizardOpen || editingApplication}<ApplicationWizard
    resources={availableResources}
    resourceScopeTypes={applicationResourceScopeTypes}
    {users}
    applications={workspace?.applications ?? []}
    projects={allProjects.map((project) => ({
      id: project.id,
      name: project.name,
      icon: project.icon
    }))}
    topology={projectTopology}
    initialTab={applicationWizardTab}
    initialApplication={editingApplication}
    initialRuntimeKind={topologyComponentIntent?.kind === 'application'
      ? topologyComponentIntent.runtimeKind
      : ''}
    onSave={saveApplications}
    onUpdate={updateApplication}
    {onResourceCreated}
    onRequestResourceCreate={(kind: RuntimeResourceKind, instanceId?: string) =>
      onRequestResourceCreate({
        id: randomId(),
        kind,
        scopeId: selectedProject?.scope.id ?? selectedScopeId,
        ...(instanceId ? { instanceId } : {})
      })}
    {resourceCreationResult}
    onCancel={() => {
      applicationWizardOpen = false;
      editingApplication = null;
      topologyComponentIntent = null;
    }}
  />{/if}
{#if deletingProject}
  <DeleteConfirmationDialog
    title={`删除项目“${deletingProject.name}”？`}
    name={deletingProject.name}
    description="将删除该项目的应用、实例、授权和范围配置。用户账号、审计记录及关联资源不会被删除；资源将保留至所属团队范围。"
    impact="此操作不可撤销。"
    {busy}
    onCancel={() => (deletingProject = null)}
    onConfirm={confirmProjectDelete}
  />
{/if}
{#if deletingApplications.length}
  <DeleteConfirmationDialog
    title={`删除 ${deletingApplications.length} 个应用？`}
    name={deletingApplications
      .map((application) => application.name)
      .join('、')}
    description="将删除所选应用及其实例和资源绑定。关联资源、项目、用户和历史审计记录不会被删除。"
    impact="此操作不可撤销。"
    {busy}
    onCancel={() => (deletingApplications = [])}
    onConfirm={confirmApplicationDelete}
  />
{/if}
