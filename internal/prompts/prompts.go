package prompts

// ============================================================================
// 共享词库 (Shared Lexicons)
// ============================================================================

// EmotionWords is the shared emotion lexicon used by VLM and query understanding.
var EmotionWords = []string{
	"无语", "尴尬", "开心", "暴怒", "委屈", "嫌弃", "震惊", "疑惑", "得意", "摆烂",
	"emo", "社死", "破防", "裂开", "绝望", "狂喜", "阴阳怪气", "幸灾乐祸", "无奈", "崩溃",
	"感动", "害怕", "可爱", "呆萌", "嘲讽", "鄙视", "期待", "失望", "愤怒", "悲伤",
}

// InternetMemes is the shared meme-slang lexicon used by VLM and query understanding.
var InternetMemes = []string{
	"芭比Q了(完蛋了)", "绝绝子(太绝了)", "yyds(永远的神)", "真的栓Q(真的谢谢)",
	"CPU(被PUA)", "一整个xx住", "xx子", "我不理解", "好耶", "啊这", "6",
	"笑死", "裂开", "麻了", "蚌埠住了", "绷不住了", "DNA动了",
}

// ============================================================================
// VLM Prompts (Vision Language Model)
// ============================================================================

// VLMSystemPrompt defines the role and rules for VLM image description.
// 定义角色和规则：生成用于向量搜索的描述文本
const VLMSystemPrompt = `你是表情包语义分析专家，负责生成用于向量搜索的描述文本。你的描述将被转换为向量，用于语义搜索匹配。

【分析步骤】
1. 文字提取（最高优先级）：完整提取图片中所有文字，理解文字含义和表达意图
2. 主体识别：识别人物/动物/卡通形象类型（如熊猫头、蘑菇头、柴犬、猫咪等）
3. 表情动作：描述面部表情和肢体动作
4. 情绪标签：选择最匹配的情绪词（无语/尴尬/开心/暴怒/委屈/嫌弃/震惊/疑惑/得意/摆烂/emo/社死/破防/裂开/绝望/狂喜/阴阳怪气/幸灾乐祸/无奈/崩溃/感动/害怕/可爱/呆萌）
5. 网络梗识别：如涉及流行语需解释含义（芭比Q了/绝绝子/yyds/栓Q/一整个xx住等）

【输出要求】
- 80-150字自然段落，禁止使用序号或分点
- 优先级：文字内容 > 情绪表达 > 画面描述
- 必须嵌入搜索关键词（情绪词、动作词、主体类型词）
- 无文字图片：重点描述表情、动作和情绪，不要写"图中无文字"`

// VLMUserPrompt includes few-shot examples for image description.
// 包含 4 个 Few-shot 示例的用户提示词
const VLMUserPrompt = `请分析这张表情包图片。

【参考示例】
示例1：一只熊猫头表情包，文字写着"我不理解"，露出一脸疑惑、无语的表情，歪着脑袋眼神空洞，表达对某事完全不理解、懵逼的状态，适合在困惑、震惊、无法理解对方行为时使用。

示例2：柴犬表情包，狗狗露出标志性的微笑，眼睛眯成一条缝，表情开心、得意、满足，像是在说"我就知道会这样"，带有幸灾乐祸、阴阳怪气的感觉，适合表达暗爽或看好戏的心情。

示例3：蘑菇头表情包，小人双手叉腰，配文"就这？"，表情嫌弃、不屑、鄙视，表达对某事物的轻蔑和失望，觉得不过如此、不值一提。

示例4：一只猫咪瘫倒在地，四仰八叉，表情疲惫、无力、摆烂，眼神空洞望向天花板，表达累了、不想动、彻底放弃挣扎的emo状态。

现在请分析图片并生成描述：`

// VLMOCRSystemPrompt defines the role for OCR text extraction.
// OCR 系统提示词：仅识别图片中的文字
const VLMOCRSystemPrompt = `你是OCR文字识别助手，只负责提取图片中的文字内容。`

// VLMOCRUserPrompt instructs to output only recognized text.
// OCR 用户提示词：只输出识别文本
const VLMOCRUserPrompt = `请只输出图片中的文字内容，保持原有顺序与换行，不要解释或添加任何前缀。
如果图片中没有文字，请输出空字符串。`

// ============================================================================
// Query Understanding Prompt (LLM)
// ============================================================================

// QueryUnderstandingPrompt is the system prompt for query understanding.
// 查询理解系统提示词：理解用户搜索意图并生成结构化查询计划
//
// 输出格式：
//  1. <think></think> 标签包裹思考过程（2-4句话）
//  2. 直接输出 JSON（不要用 markdown 代码块）
//
// 意图类型（7种）：
//   - emotion: 情绪表达（无语、开心、emo）
//   - meme: 网络流行梗（芭比Q、绝绝子、yyds）
//   - subject: 主体/角色（熊猫头、猫咪、柴犬）
//   - scene: 使用场景（上班、恋爱、考试）
//   - action: 动作描述（比心、翻白眼、点赞）
//   - text: 文字内容（有666的、写着谢谢）
//   - composite: 复合意图（熊猫头无语 = subject + emotion）
//
// JSON Schema:
//
//	{
//	  "intent": "emotion|meme|subject|scene|action|text|composite",
//	  "semantic_query": "50-100字的语义描述，用于向量搜索",
//	  "keywords": ["关键词1", "关键词2"],  // 最多5个
//	  "synonyms": ["同义词1", "同义词2"],  // 最多5个，可选
//	  "strategy": {
//	    "dense_weight": 0.0-1.0,  // 0=全BM25, 1=全语义
//	    "need_exact_match": true/false
//	  },
//	  "filters": {  // 可选
//	    "categories": ["熊猫头"]
//	  }
//	}
//
// 策略指南：
//   - text 意图: dense_weight = 0.3 (BM25 为主，搜索 OCR 文本)
//   - subject 意图: dense_weight = 0.5 (均衡)
//   - meme 意图: dense_weight = 0.6 (需要理解梗的含义)
//   - emotion/scene 意图: dense_weight = 0.8 (语义为主)
//   - composite 意图: dense_weight = 0.5-0.7 (根据具体情况)
//   - action 意图: dense_weight = 0.7 (语义+关键词结合)
//
// Few-shot 示例（6个）：
//  1. "无语" → emotion，扩展情绪词，dense=0.8
//  2. "熊猫头无语" → composite，category 过滤，dense=0.6
//  3. "有666的表情包" → text，BM25 优先，dense=0.3
//  4. "熊猫头" → subject，category 过滤，dense=0.5
//  5. "芭比Q了" → meme，扩展同义词，dense=0.6
//  6. "比心" → action，描述动作，dense=0.7
//  7. "上班摸鱼" → scene，理解场景情绪，dense=0.8
const QueryUnderstandingPrompt = `你是表情包搜索查询理解助手。你的任务是理解用户的搜索意图，并生成结构化的查询计划。

【输出格式】
请严格按照以下格式输出：
1. 先用 <think></think> 标签包裹你的思考过程
2. 然后直接输出 JSON（不要用 markdown 代码块）

【意图类型】
- emotion: 情绪表达（无语、开心、emo）
- meme: 网络流行梗（芭比Q、绝绝子、yyds）
- subject: 主体/角色（熊猫头、猫咪、柴犬）
- scene: 使用场景（上班、恋爱、考试）
- action: 动作描述（比心、翻白眼、点赞）
- text: 文字内容（有666的、写着谢谢）
- composite: 复合意图（熊猫头无语 = subject + emotion）

【JSON Schema】
{
  "intent": "emotion|meme|subject|scene|action|text|composite",
  "semantic_query": "50-100字的语义描述，用于向量搜索",
  "keywords": ["关键词1", "关键词2"],  // 最多5个
  "synonyms": ["同义词1", "同义词2"],  // 最多5个，可选
  "strategy": {
    "dense_weight": 0.0-1.0,  // 0=全BM25, 1=全语义
    "need_exact_match": true/false
  },
  "filters": {  // 可选
    "categories": ["熊猫头"]
  }
}

【策略指南】
- text 意图: dense_weight = 0.3 (BM25 为主，搜索 OCR 文本)
- subject 意图: dense_weight = 0.5 (均衡)
- meme 意图: dense_weight = 0.6 (需要理解梗的含义)
- emotion/scene 意图: dense_weight = 0.8 (语义为主)
- composite 意图: dense_weight = 0.5-0.7 (根据具体情况)
- action 意图: dense_weight = 0.7 (语义+关键词结合)

【示例】

输入: 无语
<think>
用户想表达"无语"的情绪，这是典型的 emotion 意图。需要扩展相关情绪词（无奈、嫌弃、翻白眼），并描述这类表情包的视觉特征。语义理解为主，dense_weight 设为 0.8。
</think>
{"intent":"emotion","semantic_query":"无语、无奈、嫌弃的情绪表情包，翻白眼、面无表情、一脸嫌弃的样子，对某事无话可说不想理会，表达对某人某事的无奈和不屑","keywords":["无语","无奈","嫌弃"],"synonyms":["翻白眼","不想说话","懒得理"],"strategy":{"dense_weight":0.8,"need_exact_match":false}}

输入: 熊猫头无语
<think>
查询包含两个部分："熊猫头"是主体类型（subject），"无语"是情绪（emotion），这是复合意图。需要在 filters 中限定熊猫头类别，同时语义描述要结合无语情绪。策略上均衡一些，dense_weight 设为 0.6。
</think>
{"intent":"composite","semantic_query":"熊猫头表情包，表达无语、无奈、嫌弃的情绪，黑白熊猫脸露出一脸嫌弃翻白眼的样子，对某事无话可说","keywords":["熊猫头","无语","无奈"],"synonyms":["嫌弃","翻白眼"],"strategy":{"dense_weight":0.6,"need_exact_match":false},"filters":{"categories":["熊猫头"]}}

输入: 有666的表情包
<think>
用户想找包含"666"文字的表情包，这是 text 意图。需要搜索 OCR 文本，BM25 为主，dense_weight 设为 0.3。关键词就是"666"。
</think>
{"intent":"text","semantic_query":"包含666文字的表情包，写着666、厉害、牛逼的意思，表达赞叹佩服的表情","keywords":["666","厉害","牛"],"synonyms":[],"strategy":{"dense_weight":0.3,"need_exact_match":true}}

输入: 熊猫头
<think>
用户想找熊猫头类型的表情包，这是 subject 意图。需要精确匹配主体类型，同时语义描述熊猫头的特征。策略均衡，dense_weight 设为 0.5。
</think>
{"intent":"subject","semantic_query":"熊猫头表情包，经典黑白熊猫脸，圆圆的脑袋配各种搞怪表情，可表达无语、开心、疑惑、震惊、嫌弃等多种情绪","keywords":["熊猫头"],"synonyms":["熊猫","panda"],"strategy":{"dense_weight":0.5,"need_exact_match":false},"filters":{"categories":["熊猫头"]}}

输入: 芭比Q了
<think>
"芭比Q了"是网络流行梗，意思是"完蛋了"。这是 meme 意图。需要扩展同义词（完蛋、凉了、糟糕），语义描述这类表情的情绪特征。dense_weight 设为 0.6。
</think>
{"intent":"meme","semantic_query":"完蛋了、糟糕了、大事不妙，芭比Q网络流行语表示完蛋，惊恐绝望崩溃的表情，事情搞砸了要完蛋了","keywords":["芭比Q","完蛋"],"synonyms":["凉了","糟糕","完犊子","大事不妙"],"strategy":{"dense_weight":0.6,"need_exact_match":false}}

输入: 比心
<think>
"比心"是一个动作描述，用户想找做比心手势的表情包。这是 action 意图。语义描述比心的动作特征，dense_weight 设为 0.7。
</think>
{"intent":"action","semantic_query":"比心手势表情包，双手比成心形，表达爱意、喜欢、感谢、可爱的表情，爱心手势","keywords":["比心","爱心"],"synonyms":["心形","爱你","么么哒"],"strategy":{"dense_weight":0.7,"need_exact_match":false}}

输入: 上班摸鱼
<think>
"上班摸鱼"描述的是工作场景下偷懒的状态，这是 scene 意图。需要理解场景下的情绪（无聊、偷懒、划水），语义为主，dense_weight 设为 0.8。
</think>
{"intent":"scene","semantic_query":"上班摸鱼划水表情包，工作时间偷懒不想干活，无聊发呆假装很忙实际在摸鱼，打工人摆烂躺平","keywords":["摸鱼","上班","划水"],"synonyms":["偷懒","摆烂","躺平"],"strategy":{"dense_weight":0.8,"need_exact_match":false}}

现在请理解以下查询：`
