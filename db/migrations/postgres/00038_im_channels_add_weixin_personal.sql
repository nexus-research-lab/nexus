-- +goose Up
DO $$
BEGIN
    IF to_regclass('public.im_channel_configs') IS NOT NULL THEN
        ALTER TABLE im_channel_configs
            DROP CONSTRAINT IF EXISTS ck_im_channel_configs_channel_type;

        ALTER TABLE im_channel_configs
            ADD CONSTRAINT ck_im_channel_configs_channel_type CHECK (channel_type IN ('dingtalk', 'wechat', 'weixin-personal', 'feishu', 'telegram', 'discord'));
    END IF;

    IF to_regclass('public.im_pairings') IS NOT NULL THEN
        ALTER TABLE im_pairings
            DROP CONSTRAINT IF EXISTS ck_im_pairings_channel_type;

        ALTER TABLE im_pairings
            ADD CONSTRAINT ck_im_pairings_channel_type CHECK (channel_type IN ('dingtalk', 'wechat', 'weixin-personal', 'feishu', 'telegram', 'discord'));
    END IF;
END $$;

-- +goose Down
DO $$
BEGIN
    IF to_regclass('public.im_pairings') IS NOT NULL THEN
        DELETE FROM im_pairings WHERE channel_type = 'weixin-personal';

        ALTER TABLE im_pairings
            DROP CONSTRAINT IF EXISTS ck_im_pairings_channel_type;

        ALTER TABLE im_pairings
            ADD CONSTRAINT ck_im_pairings_channel_type CHECK (channel_type IN ('dingtalk', 'wechat', 'feishu', 'telegram', 'discord'));
    END IF;

    IF to_regclass('public.im_channel_configs') IS NOT NULL THEN
        DELETE FROM im_channel_configs WHERE channel_type = 'weixin-personal';

        ALTER TABLE im_channel_configs
            DROP CONSTRAINT IF EXISTS ck_im_channel_configs_channel_type;

        ALTER TABLE im_channel_configs
            ADD CONSTRAINT ck_im_channel_configs_channel_type CHECK (channel_type IN ('dingtalk', 'wechat', 'feishu', 'telegram', 'discord'));
    END IF;
END $$;
