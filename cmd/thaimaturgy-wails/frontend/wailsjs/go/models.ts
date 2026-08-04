export namespace domain {
	
	export class AbilityScores {
	    str: number;
	    dex: number;
	    con: number;
	    int: number;
	    wis: number;
	    cha: number;
	
	    static createFrom(source: any = {}) {
	        return new AbilityScores(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.str = source["str"];
	        this.dex = source["dex"];
	        this.con = source["con"];
	        this.int = source["int"];
	        this.wis = source["wis"];
	        this.cha = source["cha"];
	    }
	}
	export class Action {
	    name: string;
	    description?: string;
	    to_hit?: string;
	    damage?: string;
	
	    static createFrom(source: any = {}) {
	        return new Action(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.to_hit = source["to_hit"];
	        this.damage = source["damage"];
	    }
	}
	export class ImageRef {
	    id: string;
	    path: string;
	    kind?: string;
	    description?: string;
	
	    static createFrom(source: any = {}) {
	        return new ImageRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	        this.kind = source["kind"];
	        this.description = source["description"];
	    }
	}
	export class LoreEntry {
	    title: string;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new LoreEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.content = source["content"];
	    }
	}
	export class Faction {
	    id: string;
	    name: string;
	    description?: string;
	    goals?: string;
	
	    static createFrom(source: any = {}) {
	        return new Faction(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.goals = source["goals"];
	    }
	}
	export class TableRow {
	    roll?: string;
	    cells?: string[];
	
	    static createFrom(source: any = {}) {
	        return new TableRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.roll = source["roll"];
	        this.cells = source["cells"];
	    }
	}
	export class Table {
	    id: string;
	    name: string;
	    description?: string;
	    dice?: string;
	    headers?: string[];
	    rows?: TableRow[];
	
	    static createFrom(source: any = {}) {
	        return new Table(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.dice = source["dice"];
	        this.headers = source["headers"];
	        this.rows = this.convertValues(source["rows"], TableRow);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Item {
	    id: string;
	    name: string;
	    description?: string;
	    rarity?: string;
	    mechanics?: string;
	    image?: string;
	    image_ids?: string[];
	
	    static createFrom(source: any = {}) {
	        return new Item(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.rarity = source["rarity"];
	        this.mechanics = source["mechanics"];
	        this.image = source["image"];
	        this.image_ids = source["image_ids"];
	    }
	}
	export class Outcome {
	    condition: string;
	    result: string;
	
	    static createFrom(source: any = {}) {
	        return new Outcome(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.condition = source["condition"];
	        this.result = source["result"];
	    }
	}
	export class Event {
	    id: string;
	    name: string;
	    trigger?: string;
	    description?: string;
	    read_aloud?: string;
	    dm_notes?: string;
	    consequences?: string;
	    outcomes?: Outcome[];
	
	    static createFrom(source: any = {}) {
	        return new Event(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.trigger = source["trigger"];
	        this.description = source["description"];
	        this.read_aloud = source["read_aloud"];
	        this.dm_notes = source["dm_notes"];
	        this.consequences = source["consequences"];
	        this.outcomes = this.convertValues(source["outcomes"], Outcome);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class StatBlock {
	    ac?: number;
	    max_hp?: number;
	    speed?: string;
	    abilities?: AbilityScores;
	    cr?: string;
	    skills?: string[];
	    traits?: string[];
	    actions?: Action[];
	
	    static createFrom(source: any = {}) {
	        return new StatBlock(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ac = source["ac"];
	        this.max_hp = source["max_hp"];
	        this.speed = source["speed"];
	        this.abilities = this.convertValues(source["abilities"], AbilityScores);
	        this.cr = source["cr"];
	        this.skills = source["skills"];
	        this.traits = source["traits"];
	        this.actions = this.convertValues(source["actions"], Action);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class NPC {
	    id: string;
	    name: string;
	    role?: string;
	    appearance?: string;
	    personality?: string;
	    motivations?: string;
	    secrets?: string;
	    voice?: string;
	    knowledge?: string[];
	    sample_dialogue?: string[];
	    disposition?: string;
	    stat_block?: StatBlock;
	    image?: string;
	    image_ids?: string[];
	    default_location?: string;
	
	    static createFrom(source: any = {}) {
	        return new NPC(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.role = source["role"];
	        this.appearance = source["appearance"];
	        this.personality = source["personality"];
	        this.motivations = source["motivations"];
	        this.secrets = source["secrets"];
	        this.voice = source["voice"];
	        this.knowledge = source["knowledge"];
	        this.sample_dialogue = source["sample_dialogue"];
	        this.disposition = source["disposition"];
	        this.stat_block = this.convertValues(source["stat_block"], StatBlock);
	        this.image = source["image"];
	        this.image_ids = source["image_ids"];
	        this.default_location = source["default_location"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Feature {
	    name: string;
	    description?: string;
	    skill?: string;
	    dc?: number;
	    success?: string;
	    failure?: string;
	
	    static createFrom(source: any = {}) {
	        return new Feature(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.skill = source["skill"];
	        this.dc = source["dc"];
	        this.success = source["success"];
	        this.failure = source["failure"];
	    }
	}
	export class Encounter {
	    name: string;
	    description?: string;
	    creatures?: string[];
	    difficulty?: string;
	    tactics?: string;
	
	    static createFrom(source: any = {}) {
	        return new Encounter(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.creatures = source["creatures"];
	        this.difficulty = source["difficulty"];
	        this.tactics = source["tactics"];
	    }
	}
	export class Exit {
	    to: string;
	    direction?: string;
	    description?: string;
	    locked?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Exit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.to = source["to"];
	        this.direction = source["direction"];
	        this.description = source["description"];
	        this.locked = source["locked"];
	    }
	}
	export class Room {
	    id: string;
	    name: string;
	    read_aloud?: string;
	    dm_notes?: string;
	    image?: string;
	    image_ids?: string[];
	    npc_ids?: string[];
	    event_ids?: string[];
	    exits?: Exit[];
	    encounters?: Encounter[];
	    treasure?: string[];
	    features?: Feature[];
	
	    static createFrom(source: any = {}) {
	        return new Room(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.read_aloud = source["read_aloud"];
	        this.dm_notes = source["dm_notes"];
	        this.image = source["image"];
	        this.image_ids = source["image_ids"];
	        this.npc_ids = source["npc_ids"];
	        this.event_ids = source["event_ids"];
	        this.exits = this.convertValues(source["exits"], Exit);
	        this.encounters = this.convertValues(source["encounters"], Encounter);
	        this.treasure = source["treasure"];
	        this.features = this.convertValues(source["features"], Feature);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Zone {
	    id: string;
	    name: string;
	    overview?: string;
	    description?: string;
	    map_image?: string;
	    image_ids?: string[];
	    rooms?: Room[];
	    connections?: string[];
	
	    static createFrom(source: any = {}) {
	        return new Zone(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.overview = source["overview"];
	        this.description = source["description"];
	        this.map_image = source["map_image"];
	        this.image_ids = source["image_ids"];
	        this.rooms = this.convertValues(source["rooms"], Room);
	        this.connections = source["connections"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Adventure {
	    schema_version: string;
	    id: string;
	    title: string;
	    author?: string;
	    system?: string;
	    language?: string;
	    summary?: string;
	    context?: string;
	    background?: string;
	    introduction?: string;
	    conclusion?: string;
	    hooks?: string[];
	    zones?: Zone[];
	    npcs?: NPC[];
	    events?: Event[];
	    items?: Item[];
	    tables?: Table[];
	    factions?: Faction[];
	    lore?: LoreEntry[];
	    images?: ImageRef[];
	    meta?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new Adventure(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema_version = source["schema_version"];
	        this.id = source["id"];
	        this.title = source["title"];
	        this.author = source["author"];
	        this.system = source["system"];
	        this.language = source["language"];
	        this.summary = source["summary"];
	        this.context = source["context"];
	        this.background = source["background"];
	        this.introduction = source["introduction"];
	        this.conclusion = source["conclusion"];
	        this.hooks = source["hooks"];
	        this.zones = this.convertValues(source["zones"], Zone);
	        this.npcs = this.convertValues(source["npcs"], NPC);
	        this.events = this.convertValues(source["events"], Event);
	        this.items = this.convertValues(source["items"], Item);
	        this.tables = this.convertValues(source["tables"], Table);
	        this.factions = this.convertValues(source["factions"], Faction);
	        this.lore = this.convertValues(source["lore"], LoreEntry);
	        this.images = this.convertValues(source["images"], ImageRef);
	        this.meta = source["meta"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class InventoryItem {
	    name: string;
	    quantity: number;
	    weight?: number;
	    equipped?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new InventoryItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.quantity = source["quantity"];
	        this.weight = source["weight"];
	        this.equipped = source["equipped"];
	    }
	}
	export class Skill {
	    name: string;
	    ability: number;
	    proficient: boolean;
	    expert: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Skill(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.ability = source["ability"];
	        this.proficient = source["proficient"];
	        this.expert = source["expert"];
	    }
	}
	export class Character {
	    name: string;
	    race: string;
	    class: string;
	    level: number;
	    background: string;
	    alignment?: string;
	    abilities: AbilityScores;
	    max_hp: number;
	    current_hp: number;
	    temp_hp?: number;
	    ac: number;
	    initiative: number;
	    speed: number;
	    proficiency_bonus: number;
	    skills: Skill[];
	    inventory: InventoryItem[];
	    conditions: string[];
	    gold: number;
	    xp: number;
	    notes?: string;
	
	    static createFrom(source: any = {}) {
	        return new Character(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.race = source["race"];
	        this.class = source["class"];
	        this.level = source["level"];
	        this.background = source["background"];
	        this.alignment = source["alignment"];
	        this.abilities = this.convertValues(source["abilities"], AbilityScores);
	        this.max_hp = source["max_hp"];
	        this.current_hp = source["current_hp"];
	        this.temp_hp = source["temp_hp"];
	        this.ac = source["ac"];
	        this.initiative = source["initiative"];
	        this.speed = source["speed"];
	        this.proficiency_bonus = source["proficiency_bonus"];
	        this.skills = this.convertValues(source["skills"], Skill);
	        this.inventory = this.convertValues(source["inventory"], InventoryItem);
	        this.conditions = source["conditions"];
	        this.gold = source["gold"];
	        this.xp = source["xp"];
	        this.notes = source["notes"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ToolCall {
	    id: string;
	    type: string;
	    // Go type: struct { Name string "json:\"name\""; Arguments string "json:\"arguments\"" }
	    function: any;
	
	    static createFrom(source: any = {}) {
	        return new ToolCall(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.function = this.convertValues(source["function"], Object);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Message {
	    id: string;
	    role: string;
	    content: string;
	    name?: string;
	    tool_calls?: ToolCall[];
	    tool_call_id?: string;
	    // Go type: time
	    timestamp: any;
	
	    static createFrom(source: any = {}) {
	        return new Message(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.role = source["role"];
	        this.content = source["content"];
	        this.name = source["name"];
	        this.tool_calls = this.convertValues(source["tool_calls"], ToolCall);
	        this.tool_call_id = source["tool_call_id"];
	        this.timestamp = this.convertValues(source["timestamp"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Conversation {
	    messages: Message[];
	    max_size: number;
	
	    static createFrom(source: any = {}) {
	        return new Conversation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.messages = this.convertValues(source["messages"], Message);
	        this.max_size = source["max_size"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	
	
	
	
	
	export class LogEntry {
	    type: string;
	    message: string;
	    data?: Record<string, any>;
	    // Go type: time
	    timestamp: any;
	
	    static createFrom(source: any = {}) {
	        return new LogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.message = source["message"];
	        this.data = source["data"];
	        this.timestamp = this.convertValues(source["timestamp"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	export class NPCStatus {
	    met: boolean;
	    disposition?: string;
	    alive: boolean;
	    notes?: string;
	
	    static createFrom(source: any = {}) {
	        return new NPCStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.met = source["met"];
	        this.disposition = source["disposition"];
	        this.alive = source["alive"];
	        this.notes = source["notes"];
	    }
	}
	
	export class PartyMember {
	    name: string;
	    class?: string;
	    level?: number;
	    current_hp?: number;
	    max_hp?: number;
	    ac?: number;
	    notes?: string;
	
	    static createFrom(source: any = {}) {
	        return new PartyMember(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.class = source["class"];
	        this.level = source["level"];
	        this.current_hp = source["current_hp"];
	        this.max_hp = source["max_hp"];
	        this.ac = source["ac"];
	        this.notes = source["notes"];
	    }
	}
	export class PlayerSlot {
	    display_name: string;
	    character_name: string;
	
	    static createFrom(source: any = {}) {
	        return new PlayerSlot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.display_name = source["display_name"];
	        this.character_name = source["character_name"];
	    }
	}
	export class QuestProgress {
	    id: string;
	    name: string;
	    status: string;
	    notes?: string;
	
	    static createFrom(source: any = {}) {
	        return new QuestProgress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.status = source["status"];
	        this.notes = source["notes"];
	    }
	}
	
	export class RoundAction {
	    player_id: string;
	    display_name: string;
	    character_name: string;
	    text: string;
	    // Go type: time
	    at: any;
	
	    static createFrom(source: any = {}) {
	        return new RoundAction(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.player_id = source["player_id"];
	        this.display_name = source["display_name"];
	        this.character_name = source["character_name"];
	        this.text = source["text"];
	        this.at = this.convertValues(source["at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SessionLog {
	    entries: LogEntry[];
	    max_size: number;
	
	    static createFrom(source: any = {}) {
	        return new SessionLog(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.entries = this.convertValues(source["entries"], LogEntry);
	        this.max_size = source["max_size"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TurnRound {
	    actions: RoundAction[];
	
	    static createFrom(source: any = {}) {
	        return new TurnRound(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.actions = this.convertValues(source["actions"], RoundAction);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SessionState {
	    name: string;
	    adventure_id: string;
	    adventure_title: string;
	    current_zone?: string;
	    current_room?: string;
	    visited_rooms?: Record<string, boolean>;
	    known_npcs?: Record<string, NPCStatus>;
	    triggered_events?: Record<string, boolean>;
	    flags?: Record<string, boolean>;
	    variables?: Record<string, string>;
	    party?: PartyMember[];
	    quests?: QuestProgress[];
	    mode?: string;
	    characters?: Character[];
	    pc?: Character;
	    players?: Record<string, PlayerSlot>;
	    round?: TurnRound;
	    started?: boolean;
	    log?: SessionLog;
	    summary?: string;
	    conversation?: Conversation;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    updated_at: any;
	    play_time: number;
	
	    static createFrom(source: any = {}) {
	        return new SessionState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.adventure_id = source["adventure_id"];
	        this.adventure_title = source["adventure_title"];
	        this.current_zone = source["current_zone"];
	        this.current_room = source["current_room"];
	        this.visited_rooms = source["visited_rooms"];
	        this.known_npcs = this.convertValues(source["known_npcs"], NPCStatus, true);
	        this.triggered_events = source["triggered_events"];
	        this.flags = source["flags"];
	        this.variables = source["variables"];
	        this.party = this.convertValues(source["party"], PartyMember);
	        this.quests = this.convertValues(source["quests"], QuestProgress);
	        this.mode = source["mode"];
	        this.characters = this.convertValues(source["characters"], Character);
	        this.pc = this.convertValues(source["pc"], Character);
	        this.players = this.convertValues(source["players"], PlayerSlot, true);
	        this.round = this.convertValues(source["round"], TurnRound);
	        this.started = source["started"];
	        this.log = this.convertValues(source["log"], SessionLog);
	        this.summary = source["summary"];
	        this.conversation = this.convertValues(source["conversation"], Conversation);
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.updated_at = this.convertValues(source["updated_at"], null);
	        this.play_time = source["play_time"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	
	
	

}

export namespace storage {
	
	export class AdventureInfo {
	    id: string;
	    title: string;
	    author: string;
	    system: string;
	    dir: string;
	
	    static createFrom(source: any = {}) {
	        return new AdventureInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.author = source["author"];
	        this.system = source["system"];
	        this.dir = source["dir"];
	    }
	}
	export class SessionInfo {
	    name: string;
	    adventure_id: string;
	    adventure_title: string;
	    current_room: string;
	    play_time: any;
	    modified_at: any;
	
	    static createFrom(source: any = {}) {
	        return new SessionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.adventure_id = source["adventure_id"];
	        this.adventure_title = source["adventure_title"];
	        this.current_room = source["current_room"];
	        this.play_time = source["play_time"];
	        this.modified_at = source["modified_at"];
	    }
	}

}

export namespace wailsapp {
	
	export class NavRef {
	    label: string;
	    uid: string;
	
	    static createFrom(source: any = {}) {
	        return new NavRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.uid = source["uid"];
	    }
	}
	export class NavGroup {
	    title: string;
	    refs: NavRef[];
	
	    static createFrom(source: any = {}) {
	        return new NavGroup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.refs = this.convertValues(source["refs"], NavRef);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DetailPayload {
	    uid: string;
	    kind: string;
	    title: string;
	    markdown: string;
	    images?: string[];
	    groups?: NavGroup[];
	    actions?: string[];
	
	    static createFrom(source: any = {}) {
	        return new DetailPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uid = source["uid"];
	        this.kind = source["kind"];
	        this.title = source["title"];
	        this.markdown = source["markdown"];
	        this.images = source["images"];
	        this.groups = this.convertValues(source["groups"], NavGroup);
	        this.actions = source["actions"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TreeNode {
	    uid: string;
	    label: string;
	    kind: string;
	    children?: TreeNode[];
	
	    static createFrom(source: any = {}) {
	        return new TreeNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uid = source["uid"];
	        this.label = source["label"];
	        this.kind = source["kind"];
	        this.children = this.convertValues(source["children"], TreeNode);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SessionPayload {
	    state?: domain.SessionState;
	    adventure?: domain.Adventure;
	    current_room?: domain.Room;
	    current_zone?: domain.Zone;
	    tree: TreeNode[];
	    detail?: DetailPayload;
	
	    static createFrom(source: any = {}) {
	        return new SessionPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = this.convertValues(source["state"], domain.SessionState);
	        this.adventure = this.convertValues(source["adventure"], domain.Adventure);
	        this.current_room = this.convertValues(source["current_room"], domain.Room);
	        this.current_zone = this.convertValues(source["current_zone"], domain.Zone);
	        this.tree = this.convertValues(source["tree"], TreeNode);
	        this.detail = this.convertValues(source["detail"], DetailPayload);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ConfigPayload {
	    provider: string;
	    model: string;
	    edit_model: string;
	    run_model: string;
	    language: string;
	    import_language: string;
	    temperature: number;
	    max_tokens: number;
	    import_max_output_tokens: number;
	    oracle_max_tool_iterations: number;
	    request_timeout_seconds: number;
	    auto_save: boolean;
	    auto_save_interval: number;
	    tts_enabled: boolean;
	    tts_voice: string;
	    tts_model: string;
	    tts_speed: number;
	    openai_api_key_set: boolean;
	    anthropic_api_key_set: boolean;
	    gemini_api_key_set: boolean;
	    telegram_bot_token_set: boolean;
	    telegram_chat_id: number;
	    configured: boolean;
	    config_path: string;
	    data_path: string;
	
	    static createFrom(source: any = {}) {
	        return new ConfigPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.model = source["model"];
	        this.edit_model = source["edit_model"];
	        this.run_model = source["run_model"];
	        this.language = source["language"];
	        this.import_language = source["import_language"];
	        this.temperature = source["temperature"];
	        this.max_tokens = source["max_tokens"];
	        this.import_max_output_tokens = source["import_max_output_tokens"];
	        this.oracle_max_tool_iterations = source["oracle_max_tool_iterations"];
	        this.request_timeout_seconds = source["request_timeout_seconds"];
	        this.auto_save = source["auto_save"];
	        this.auto_save_interval = source["auto_save_interval"];
	        this.tts_enabled = source["tts_enabled"];
	        this.tts_voice = source["tts_voice"];
	        this.tts_model = source["tts_model"];
	        this.tts_speed = source["tts_speed"];
	        this.openai_api_key_set = source["openai_api_key_set"];
	        this.anthropic_api_key_set = source["anthropic_api_key_set"];
	        this.gemini_api_key_set = source["gemini_api_key_set"];
	        this.telegram_bot_token_set = source["telegram_bot_token_set"];
	        this.telegram_chat_id = source["telegram_chat_id"];
	        this.configured = source["configured"];
	        this.config_path = source["config_path"];
	        this.data_path = source["data_path"];
	    }
	}
	export class LibraryPayload {
	    adventures: storage.AdventureInfo[];
	    sessions: storage.SessionInfo[];
	    config: ConfigPayload;
	
	    static createFrom(source: any = {}) {
	        return new LibraryPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.adventures = this.convertValues(source["adventures"], storage.AdventureInfo);
	        this.sessions = this.convertValues(source["sessions"], storage.SessionInfo);
	        this.config = this.convertValues(source["config"], ConfigPayload);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ActionResult {
	    message: string;
	    library?: LibraryPayload;
	    session?: SessionPayload;
	
	    static createFrom(source: any = {}) {
	        return new ActionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.message = source["message"];
	        this.library = this.convertValues(source["library"], LibraryPayload);
	        this.session = this.convertValues(source["session"], SessionPayload);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	
	
	
	export class SubmitResult {
	    success: boolean;
	    message: string;
	    session?: SessionPayload;
	
	    static createFrom(source: any = {}) {
	        return new SubmitResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.message = source["message"];
	        this.session = this.convertValues(source["session"], SessionPayload);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

