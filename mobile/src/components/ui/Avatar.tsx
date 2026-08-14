import React from 'react';
import { View, Text, Image, StyleSheet, ViewStyle, ImageSourcePropType } from 'react-native';
import { colors } from '../../utils/theme';

type AvatarSize = 'sm' | 'md' | 'lg' | 'xl';

interface AvatarProps {
  source?: ImageSourcePropType | null;
  name?: string;
  size?: AvatarSize;
  style?: ViewStyle;
}

const Avatar: React.FC<AvatarProps> = ({
  source,
  name,
  size = 'md',
  style,
}) => {
  const getSize = (): number => {
    switch (size) {
      case 'sm':
        return 32;
      case 'lg':
        return 56;
      case 'xl':
        return 72;
      case 'md':
      default:
        return 40;
    }
  };

  const getFontSize = (): number => {
    switch (size) {
      case 'sm':
        return 12;
      case 'lg':
        return 20;
      case 'xl':
        return 28;
      case 'md':
      default:
        return 16;
    }
  };

  const getInitials = (): string => {
    if (!name) return '?';
    const parts = name.trim().split(' ');
    if (parts.length >= 2) {
      return `${parts[0][0]}${parts[parts.length - 1][0]}`.toUpperCase();
    }
    return name.substring(0, 2).toUpperCase();
  };

  const getColorFromName = (): string => {
    if (!name) return colors.secondary;
    const hash = name.split('').reduce((acc, char) => acc + char.charCodeAt(0), 0);
    const avatarColors = [
      '#2563eb',
      '#7c3aed',
      '#db2777',
      '#dc2626',
      '#ea580c',
      '#16a34a',
      '#0891b2',
    ];
    return avatarColors[hash % avatarColors.length];
  };

  const avatarSize = getSize();
  const fontSize = getFontSize();

  if (source) {
    return (
      <Image
        source={source}
        style={[
          styles.avatar,
          {
            width: avatarSize,
            height: avatarSize,
            borderRadius: avatarSize / 2,
          },
          style,
        ]}
      />
    );
  }

  return (
    <View
      style={[
        styles.fallback,
        {
          width: avatarSize,
          height: avatarSize,
          borderRadius: avatarSize / 2,
          backgroundColor: getColorFromName(),
        },
        style,
      ]}
    >
      <Text style={[styles.initials, { fontSize }]}>{getInitials()}</Text>
    </View>
  );
};

const styles = StyleSheet.create({
  avatar: {},
  fallback: {
    justifyContent: 'center',
    alignItems: 'center',
  },
  initials: {
    color: '#ffffff',
    fontWeight: '600',
  },
});

export default Avatar;
