-- +goose Up
CREATE TABLE member_social_preferences (
 user_id varchar(36) PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 discoverable boolean NOT NULL DEFAULT true,
 share_cover boolean NOT NULL DEFAULT false
);
CREATE TABLE member_follows (
 follower_id varchar(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 followed_id varchar(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(follower_id,followed_id), CHECK(follower_id<>followed_id)
);
CREATE INDEX member_follows_target_idx ON member_follows(followed_id);
CREATE TABLE member_friendships (
 requester_id varchar(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 addressee_id varchar(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 state text NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','accepted')),
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(requester_id,addressee_id), CHECK(requester_id<>addressee_id)
);
CREATE UNIQUE INDEX member_friendship_pair_idx ON member_friendships(LEAST(requester_id,addressee_id),GREATEST(requester_id,addressee_id));
CREATE INDEX member_friendship_addressee_idx ON member_friendships(addressee_id,state);
CREATE TABLE member_posts (
 id varchar(36) PRIMARY KEY,
 author_id varchar(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 body text NOT NULL CHECK(char_length(body) BETWEEN 1 AND 3000),
 visibility text NOT NULL DEFAULT 'members' CHECK(visibility IN ('members','friends')),
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX member_posts_feed_idx ON member_posts(created_at DESC,id DESC);
CREATE INDEX member_posts_author_idx ON member_posts(author_id,created_at DESC);
CREATE TABLE member_post_likes (
 post_id varchar(36) NOT NULL REFERENCES member_posts(id) ON DELETE CASCADE,
 user_id varchar(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 PRIMARY KEY(post_id,user_id)
);
-- +goose Down
DROP TABLE IF EXISTS member_post_likes;
DROP TABLE IF EXISTS member_posts;
DROP TABLE IF EXISTS member_friendships;
DROP TABLE IF EXISTS member_follows;
DROP TABLE IF EXISTS member_social_preferences;
