import {
  siAnthropic,
  siApache,
  siApachekafka,
  siAlibabadotcom,
  siArtifacthub,
  siDeepseek,
  siDocker,
  siDatadog,
  siElastic,
  siElasticsearch,
  siGit,
  siGitea,
  siGithub,
  siGitlab,
  siGrafana,
  siHelm,
  siHarbor,
  siGooglegemini,
  siKimi,
  siKubernetes,
  siMinimax,
  siMysql,
  siMinio,
  siMongodb,
  siMoonshotai,
  siOllama,
  siOpenrouter,
  siPostgresql,
  siPrometheus,
  siQwen,
  siRabbitmq,
  siRedis,
  siJaeger,
  siLinuxserver,
  siModelcontextprotocol,
  siNginx
} from 'simple-icons';
import chatglm from '@iconify-icons/thesvg/chatglm';
import doubao from '@iconify-icons/thesvg/doubao';
import grokXai from '@iconify-icons/thesvg/grok-xai';
import longcat from '@iconify-icons/thesvg/longcat';
import mimo from '@iconify-icons/thesvg/xiaomi-mimo';
import openaiChatgpt from '@iconify-icons/thesvg/openai-chatgpt';
import siliconflow from '@iconify-icons/thesvg/siliconcloud-siliconflow';
import zhipu from '@iconify-icons/thesvg/zhipu';

type SimpleIconData = {
  title: string;
  path: string;
};

type IconifyIconData = {
  body: string;
  width?: number;
  height?: number;
  left?: number;
  top?: number;
};

export type IconifyIconDefinition =
  | {
      id: string;
      label: string;
      aliases: string[];
      source: 'simple-icons';
      icon: SimpleIconData;
    }
  | {
      id: string;
      label: string;
      aliases: string[];
      source: 'theSVG';
      icon: IconifyIconData;
    };

function simple(
  id: string,
  label: string,
  icon: SimpleIconData,
  aliases: string[] = []
): IconifyIconDefinition {
  return { id, label, aliases, source: 'simple-icons', icon };
}

function theSVG(
  id: string,
  label: string,
  icon: IconifyIconData,
  aliases: string[] = []
): IconifyIconDefinition {
  return { id, label, aliases, source: 'theSVG', icon };
}

export const iconifyIconEntries: IconifyIconDefinition[] = [
  simple('iconify:simple-icons:artifacthub', 'Artifact Hub', siArtifacthub),
  simple('iconify:simple-icons:git', 'Git', siGit),
  simple('iconify:simple-icons:linuxserver', 'LinuxServer', siLinuxserver),
  simple('iconify:simple-icons:alibabadotcom', 'Alibaba', siAlibabadotcom),
  simple(
    'iconify:simple-icons:modelcontextprotocol',
    'Model Context Protocol',
    siModelcontextprotocol
  ),
  simple('iconify:simple-icons:apache', 'Apache', siApache),
  simple('iconify:simple-icons:nginx', 'Nginx', siNginx),
  simple('iconify:simple-icons:anthropic', 'Anthropic', siAnthropic),
  simple('iconify:simple-icons:apache-kafka', 'Apache Kafka', siApachekafka, [
    'Kafka'
  ]),
  simple('iconify:simple-icons:datadog', 'Datadog', siDatadog),
  simple('iconify:simple-icons:docker', 'Docker', siDocker),
  simple('iconify:simple-icons:elastic', 'Elastic', siElastic),
  simple(
    'iconify:simple-icons:elasticsearch',
    'Elasticsearch',
    siElasticsearch,
    ['Elastic Search']
  ),
  simple('iconify:simple-icons:gitea', 'Gitea', siGitea),
  simple('iconify:simple-icons:github', 'GitHub', siGithub),
  simple('iconify:simple-icons:gitlab', 'GitLab', siGitlab),
  simple('iconify:simple-icons:grafana', 'Grafana', siGrafana),
  simple('iconify:simple-icons:gemini', 'Gemini', siGooglegemini, [
    'Google Gemini'
  ]),
  simple('iconify:simple-icons:harbor', 'Harbor', siHarbor),
  simple('iconify:simple-icons:helm', 'Helm', siHelm),
  simple('iconify:simple-icons:jaeger', 'Jaeger', siJaeger),
  theSVG('iconify:brand:oracle', 'Oracle', {
    body: '<path fill="#ee0004" d="M46.99 31.628A7.5 7.5 0 0 0 39.5 24.5h-15A7.5 7.5 0 0 0 17 32l.01.372A7.5 7.5 0 0 0 24.5 39.5h15l.372-.01A7.5 7.5 0 0 0 47 32zm4.994.992A12.5 12.5 0 0 1 39.5 44.5h-15a12.5 12.5 0 0 1-12.484-11.88L12 32a12.5 12.5 0 0 1 12.5-12.5h15A12.5 12.5 0 0 1 52 32z"/>',
    width: 64,
    height: 64
  }),
  theSVG('iconify:brand:oceanbase', 'OceanBase', {
    body: '<path fill="currentColor" d="M13.074 19.846a23 23 0 0 1 2.11-.768l.547-.157A21.8 21.8 0 0 1 24 18.244v-2.826a24.6 24.6 0 0 0-9.142.821l-.547.156c-.234.078-.482.156-.716.234c-.469.156-.938.339-1.393.521l.872 2.683ZM23.987 6.264v8.451a25 25 0 0 0-9.467.899c-.391.117-.768.234-1.159.364c-.469.156-.938.313-1.394.443A33.6 33.6 0 0 1 .013 17.723V9.272c3.073.3 6.238.026 9.35-.86c.43-.117.846-.26 1.276-.403s.859-.286 1.276-.417a34 34 0 0 1 12.072-1.341Zm-13.061-2.11a23 23 0 0 1-2.11.768l-.547.157c-2.761.768-5.56.977-8.269.677v2.826c2.995.3 6.081.039 9.142-.821l.547-.156c.234-.078.482-.156.716-.234c.469-.156.938-.313 1.393-.521l-.872-2.683Z"/>',
    width: 24,
    height: 24
  }),
  theSVG('iconify:brand:loki', 'Grafana Loki', {
    body: '<path fill="#f15b2b" d="m139.6 464.9l-40.8 6.3l6.3 40.8l40.8-6.3zM286.9 418.6L467 390.9l-6.3-40.8l-180.1 27.7zM216.7 387.6l6.3 40.8l40.8-6.2l-6.3-40.9zM209.8 495.9l-6.3-40.8l-40.8 6.3l6.2 40.8zM95.2 448.1l40.8-6.3l-6.2-40.8l-40.8 6.3zM470.6 414l-180.2 27.7l6.3 40.8l180.1-27.7zM226.6 451.5l6.2 40.8l40.8-6.2l-6.2-40.9zM159.1 438.3l40.8-6.3l-6.2-40.8l-40.9 6.3zM85.4 384.2l17.2-2.6L52.4 55l-17.2 2.7zM109.1 380.6l17.2-2.7L72.4 27.3L55.2 30zM149.6 374.3l17.2-2.6L109.6 0L92.4 2.7zM173.2 370.7l17.3-2.7l-52.2-338.8l-17.2 2.6zM213.2 364.6l17.2-2.7l-46.3-301l-17.2 2.6zM236.9 360.9l17.2-2.6L206 45.7l-17.2 2.6z"/>',
    width: 512,
    height: 512
  }),
  theSVG('iconify:brand:tempo', 'Grafana Tempo', {
    body: '<path fill="#f05a28" d="M19.6 152.6H10c-5.5 0-10 4.5-10 10s4.5 10 10 10h9.6c5.5 0 10-4.5 10-10c-.1-5.5-4.5-10-10-10m184 145.3H194c-5.5 0-10 4.4-10 9.9s4.4 10 9.9 10h9.6c5.5 0 10-4.4 10-9.9c.1-5.6-4.3-10-9.9-10m-8-94.8c0-5.5-4.5-10-10-10h-33.3c-5.5 0-10 4.5-10 10s4.5 10 10 10h33.3c5.5-.1 10-4.5 10-10m315.7-62.6l-10.1-56.9c-2.9-21.3-21.6-37-43.1-36H68.2C53.6 47.1 41.4 58.5 40.9 73c-.1 2.3.1 4.7.7 6.9l10.7 60.5c.1.5.2 1 .3 1.4c1.3 7.4-2.5 10.4-5.8 11.5c-5.2 1.9-7.9 7.6-6 12.7c1.4 3.9 5.1 6.6 9.3 6.6h434.6c14.6.5 26.8-10.9 27.3-25.5c.1-2.1-.2-4.4-.7-6.6M378.7 368.6c-4.2-22.1-16.8-30.2-31-30.2h-103c-5.5 0-10 4.6-10 10.1c0 5.1 3.8 9.5 8.9 10c3.3.6 6.8 4.9 9 15.5l10.6 58.8c4 18.2 20 31.3 38.6 31.7l61.5-.3c14.6.7 26.9-10.5 27.6-25.1q.2-3.4-.6-6.9zm-136.2-50.1h111.8c.8 0 1.7-.1 2.5-.4c10.3-2.2 10.8-10.6 9.5-18.9L352 221c-3.9-20.3-15.7-28.4-31.2-28.4H218.4c-5.5.1-9.9 4.6-9.8 10.1c.1 5 3.8 9.1 8.7 9.8c3.4.6 7.1 5.2 9.3 16.8l10.8 59.8c1.2 4.3-1.4 8.7-5.7 9.9c-.2.1-.4.1-.7.1c-5.2 1.8-7.9 7.4-6.1 12.6c1.4 4 5.2 6.7 9.4 6.7h8z"/>',
    width: 544,
    height: 512
  }),
  theSVG('iconify:brand:alertmanager', 'Alertmanager', {
    body: '<path fill="#e6522c" d="M56.667.667C25.372.667 0 26.036 0 57.332s25.372 56.666 56.667 56.666s56.666-25.371 56.666-56.666c0-31.296-25.372-56.665-56.666-56.665m0 106.055c-8.904 0-16.123-5.948-16.123-13.283H72.79c0 7.334-7.219 13.283-16.123 13.283m26.63-17.682H30.034v-9.658h53.264v9.658Zm-.191-14.629h-52.92c-.176-.203-.356-.403-.526-.609c-5.452-6.62-6.736-10.076-7.983-13.598c-.021-.116 6.611 1.355 11.314 2.413c0 0 2.42.56 5.958 1.205c-3.397-3.982-5.414-9.044-5.414-14.218c0-11.359 8.712-21.285 5.569-29.308c3.059.249 6.331 6.456 6.552 16.161c3.252-4.494 4.613-12.701 4.613-17.733c0-5.21 3.433-11.262 6.867-11.469c-3.061 5.045.793 9.37 4.219 20.099c1.285 4.03 1.121 10.812 2.113 15.113C63.797 33.534 65.333 20.5 71 16c-2.5 5.667.37 12.758 2.333 16.167c3.167 5.5 5.087 9.667 5.087 17.548c0 5.284-1.951 10.259-5.242 14.148c3.742-.702 6.326-1.335 6.326-1.335l12.152-2.371c.001-.001-1.765 7.261-8.55 14.254"/>',
    width: 115.333,
    height: 114
  }),
  theSVG('iconify:brand:circle-letter-r', 'Circle Letter R', {
    body: '<circle cx="12" cy="12" r="9" fill="none" stroke="currentColor" stroke-width="1.8"/><text x="12" y="16" fill="currentColor" font-family="sans-serif" font-size="12" font-weight="600" text-anchor="middle">R</text>',
    width: 24,
    height: 24
  }),
  theSVG('iconify:brand:circle-letter-t', 'Circle Letter T', {
    body: '<circle cx="12" cy="12" r="9" fill="none" stroke="currentColor" stroke-width="1.8"/><text x="12" y="16" fill="currentColor" font-family="sans-serif" font-size="12" font-weight="600" text-anchor="middle">T</text>',
    width: 24,
    height: 24
  }),
  simple('iconify:simple-icons:kimi', 'Kimi', siKimi),
  simple('iconify:simple-icons:kubernetes', 'Kubernetes', siKubernetes, [
    'K8s'
  ]),
  simple('iconify:simple-icons:minimax', 'MiniMax', siMinimax),
  simple('iconify:simple-icons:minio', 'MinIO', siMinio),
  simple('iconify:simple-icons:mongodb', 'MongoDB', siMongodb),
  simple('iconify:simple-icons:moonshot', 'Moonshot', siMoonshotai, [
    'Moonshot AI'
  ]),
  simple('iconify:simple-icons:mysql', 'MySQL', siMysql),
  simple('iconify:simple-icons:ollama', 'Ollama', siOllama),
  simple('iconify:simple-icons:openrouter', 'OpenRouter', siOpenrouter),
  simple('iconify:simple-icons:postgresql', 'PostgreSQL', siPostgresql, [
    'Postgres'
  ]),
  simple('iconify:simple-icons:prometheus', 'Prometheus', siPrometheus),
  simple('iconify:simple-icons:qwen', 'Qwen', siQwen, ['通义千问']),
  simple('iconify:simple-icons:rabbitmq', 'RabbitMQ', siRabbitmq),
  simple('iconify:simple-icons:redis', 'Redis', siRedis),
  simple('iconify:simple-icons:deepseek', 'DeepSeek', siDeepseek),
  theSVG('iconify:thesvg:chatglm', 'ChatGLM', chatglm, ['Chat GLM']),
  theSVG('iconify:thesvg:zhipu', 'GLM', zhipu, ['ChatGLM', '智谱']),
  theSVG('iconify:thesvg:doubao', 'Doubao', doubao, ['豆包']),
  theSVG('iconify:thesvg:grok-xai', 'Grok', grokXai, ['xAI']),
  theSVG('iconify:thesvg:longcat', 'LongCat', longcat),
  theSVG('iconify:thesvg:openai-chatgpt', 'OpenAI', openaiChatgpt, ['ChatGPT']),
  theSVG(
    'iconify:thesvg:siliconcloud-siliconflow',
    'SiliconFlow',
    siliconflow,
    ['硅基流动']
  ),
  theSVG('iconify:thesvg:xiaomi-mimo', 'MiMo', mimo, ['Xiaomi MiMo'])
];

const entriesByID = new Map(
  iconifyIconEntries.map((entry) => [entry.id, entry])
);

const namesToIDs: Record<string, string> = {
  'Artifact Hub': 'iconify:simple-icons:artifacthub',
  LinuxServer: 'iconify:simple-icons:linuxserver',
  Alibaba: 'iconify:simple-icons:alibabadotcom',
  'Model Context Protocol': 'iconify:simple-icons:modelcontextprotocol',
  Anthropic: 'iconify:simple-icons:anthropic',
  Kafka: 'iconify:simple-icons:apache-kafka',
  Docker: 'iconify:simple-icons:docker',
  Datadog: 'iconify:simple-icons:datadog',
  Elastic: 'iconify:simple-icons:elastic',
  ElasticSearch: 'iconify:simple-icons:elasticsearch',
  Git: 'iconify:simple-icons:git',
  Gitea: 'iconify:simple-icons:gitea',
  GitHub: 'iconify:simple-icons:github',
  GitLab: 'iconify:simple-icons:gitlab',
  Grafana: 'iconify:simple-icons:grafana',
  Gemini: 'iconify:simple-icons:gemini',
  Harbor: 'iconify:simple-icons:harbor',
  Helm: 'iconify:simple-icons:helm',
  Kimi: 'iconify:simple-icons:kimi',
  Kubernetes: 'iconify:simple-icons:kubernetes',
  MiniMax: 'iconify:simple-icons:minimax',
  MySQL: 'iconify:simple-icons:mysql',
  MinIO: 'iconify:simple-icons:minio',
  MongoDB: 'iconify:simple-icons:mongodb',
  Moonshot: 'iconify:simple-icons:moonshot',
  Ollama: 'iconify:simple-icons:ollama',
  OpenAI: 'iconify:thesvg:openai-chatgpt',
  OpenRouter: 'iconify:simple-icons:openrouter',
  PostgreSQL: 'iconify:simple-icons:postgresql',
  Prometheus: 'iconify:simple-icons:prometheus',
  Qwen: 'iconify:simple-icons:qwen',
  RabbitMQ: 'iconify:simple-icons:rabbitmq',
  Redis: 'iconify:simple-icons:redis',
  Jaeger: 'iconify:simple-icons:jaeger',
  Skywalking: 'iconify:simple-icons:apache',
  Apache: 'iconify:simple-icons:apache',
  Nginx: 'iconify:simple-icons:nginx',
  Oracle: 'iconify:brand:oracle',
  OceanBase: 'iconify:brand:oceanbase',
  Loki: 'iconify:brand:loki',
  Tempo: 'iconify:brand:tempo',
  Alertmanager: 'iconify:brand:alertmanager',
  TongRDS: 'iconify:brand:circle-letter-r',
  TongHttpServer: 'iconify:brand:circle-letter-t',
  DeepSeek: 'iconify:simple-icons:deepseek'
};

export const providerBrandIconValues: Record<string, string> = {
  openai_compatible: 'iconify:thesvg:openai-chatgpt',
  anthropic: namesToIDs.Anthropic,
  deepseek: namesToIDs.DeepSeek,
  gemini: namesToIDs.Gemini,
  kimi: namesToIDs.Kimi,
  minimax: namesToIDs.MiniMax,
  ollama: namesToIDs.Ollama,
  openrouter: namesToIDs.OpenRouter,
  qwen: namesToIDs.Qwen,
  openai: 'iconify:thesvg:openai-chatgpt',
  grok: 'iconify:thesvg:grok-xai',
  glm: 'iconify:thesvg:zhipu',
  mimo: 'iconify:thesvg:xiaomi-mimo',
  longcat: 'iconify:thesvg:longcat',
  doubao: 'iconify:thesvg:doubao',
  siliconflow: 'iconify:thesvg:siliconcloud-siliconflow'
};

export function iconifyIconForValue(value: string | undefined) {
  return value ? entriesByID.get(value) : undefined;
}

export function iconifyValueForName(name: string | undefined) {
  return name ? (namesToIDs[name] ?? '') : '';
}

export function providerBrandIconValue(providerType: string | undefined) {
  return providerType ? (providerBrandIconValues[providerType] ?? '') : '';
}
