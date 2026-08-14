import React from 'react';
import { View, Text, TouchableOpacity, StyleSheet, ViewStyle } from 'react-native';
import { colors, spacing } from '../../utils/theme';

interface ListItemProps {
  title: string;
  subtitle?: string;
  rightElement?: React.ReactNode;
  leftElement?: React.ReactNode;
  onPress?: () => void;
  showDivider?: boolean;
  style?: ViewStyle;
}

const ListItem: React.FC<ListItemProps> = ({
  title,
  subtitle,
  rightElement,
  leftElement,
  onPress,
  showDivider = true,
  style,
}) => {
  const content = (
    <View style={[styles.container, style]}>
      {leftElement && <View style={styles.leftElement}>{leftElement}</View>}
      <View style={styles.contentContainer}>
        <Text style={styles.title} numberOfLines={1}>
          {title}
        </Text>
        {subtitle && (
          <Text style={styles.subtitle} numberOfLines={2}>
            {subtitle}
          </Text>
        )}
      </View>
      {rightElement && <View style={styles.rightElement}>{rightElement}</View>}
    </View>
  );

  if (onPress) {
    return (
      <TouchableOpacity onPress={onPress} activeOpacity={0.7}>
        {content}
        {showDivider && <View style={styles.divider} />}
      </TouchableOpacity>
    );
  }

  return (
    <>
      {content}
      {showDivider && <View style={styles.divider} />}
    </>
  );
};

const styles = StyleSheet.create({
  container: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingVertical: spacing.md,
    paddingHorizontal: spacing.lg,
    backgroundColor: colors.surface,
  },
  leftElement: {
    marginRight: spacing.md,
  },
  contentContainer: {
    flex: 1,
  },
  title: {
    fontSize: 16,
    fontWeight: '500',
    color: colors.text,
  },
  subtitle: {
    fontSize: 14,
    color: colors.textSecondary,
    marginTop: spacing.xs,
  },
  rightElement: {
    marginLeft: spacing.md,
  },
  divider: {
    height: 1,
    backgroundColor: colors.border,
    marginLeft: spacing.lg,
  },
});

export default ListItem;
