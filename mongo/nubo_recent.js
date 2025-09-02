db.createCollection("users_recent");
db.users_recent.createIndex({ username: 1 }, { unique: true });
db.users_recent.createIndex({ email: 1 }, { unique: true });
db.users_recent.insertOne({
    _id: UUID(),
    username: "",
    email: "",
    email_verified: false,
    phone: null,
    phone_verified: false,
    password_hash: "",
    first_name: "",
    last_name: "",
    birthdate: null,
    sex: null,
    bio: "",
    profile_picture_id: null,
    grade: 1,
    location: "",
    school: "",
    work: "",
    badges: [],
    created_at: new Date(),
    updated_at: new Date(),
    connected: false,
    last_used: new Date()
});
db.createCollection("user_settings_recent");
db.user_settings_recent.createIndex({ user_id: 1 }, { unique: true });
db.user_settings_recent.insertOne({
    _id: UUID(),
    user_id: UUID(),
    privacy: {},
    notifications: {},
    language: "",
    theme: 0,
    created_at: new Date(),
    updated_at: new Date(),
    last_used: new Date()
});
db.createCollection("sessions_recent");
db.sessions_recent.createIndex({ user_id: 1, revoked: 1 });
db.sessions_recent.insertOne({
    _id: UUID(),
    user_id: UUID(),
    refresh_token: "",
    device_info: {},
    ip: [],
    created_at: new Date(),
    expires_at: null,
    revoked: false,
    last_used: new Date()
});
db.createCollection("relations_recent");
db.relations_recent.createIndex({ primary_id: 1 });
db.relations_recent.createIndex({ secondary_id: 1 });
db.relations_recent.createIndex({ secondary_id: 1, primary_id: 1 }, { unique: true });
db.relations_recent.insertOne({
    _id: UUID(),
    primary_id: UUID(),
    secondary_id: UUID(),
    state: 1,
    created_at: new Date(),
    last_used: new Date()
});
db.createCollection("posts_recent");
db.posts_recent.createIndex({ user_id: 1, created_at: -1 });
db.posts_recent.insertOne({
    _id: UUID(),
    user_id: UUID(),
    content: "",
    media_ids: [],
    visibility: 0,
    location: "",
    created_at: new Date(),
    updated_at: new Date(),
    last_used: new Date()
});
db.createCollection("comments_recent");
db.comments_recent.createIndex({ post_id: 1, created_at: -1 });
db.comments_recent.insertOne({
    _id: UUID(),
    post_id: UUID(),
    user_id: UUID(),
    content: "",
    created_at: new Date(),
    last_used: new Date()
});
db.createCollection("likes_recent");
db.likes_recent.createIndex({ target_type: 1, target_id: 1 });
db.likes_recent.createIndex({ target_type: 1, target_id: 1, user_id: 1 }, { unique: true });
db.likes_recent.insertOne({
    _id: UUID(),
    target_type: 0,
    target_id: UUID(),
    user_id: UUID(),
    created_at: new Date(),
    last_used: new Date()
});
db.createCollection("media_recent");
db.media_recent.createIndex({ owner_id: 1 });
db.media_recent.createIndex({ created_at: 1 });
db.media_recent.insertOne({
    _id: UUID(),
    owner_id: UUID(),
    storage_path: "",
    created_at: new Date(),
    last_used: new Date()
});
db.createCollection("conversations_recent");
db.conversations_recent.createIndex({ last_message_id: 1 });

db.conversations_recent.insertOne({
    _id: UUID(),
    type: 0,
    title: "",
    last_message_id: null,
    state: 0,
    created_at: new Date(),
    last_used: new Date()
});
db.createCollection("conversation_members_recent");
db.conversation_members_recent.createIndex({ conversation_id: 1, user_id: 1 }, { unique: true });
db.conversation_members_recent.insertOne({
    _id: UUID(),
    conversation_id: UUID(),
    user_id: UUID(),
    role: 0,
    joined_at: new Date(),
    unread_count: 0,
    last_used: new Date()
});
db.createCollection("messages_recent");
db.messages_recent.createIndex({ conversation_id: 1, created_at: -1 });
db.messages_recent.insertOne({
    _id: UUID(),
    conversation_id: UUID(),
    sender_id: UUID(),
    message_type: 0,
    state: 0,
    content: "",
    attachments: {},
    created_at: new Date(),
    last_used: new Date()
});
db.createCollection("feed_cache");
db.feed_cache.createIndex({ user_id: 1, created_at: -1 });
db.feed_cache.insertOne({
    _id: UUID(),
    user_id: UUID(),
    items: [],
    created_at: new Date(),
    last_used: new Date()
});