import React from 'react';
import { Text as RNText, StyleSheet, TextStyle } from 'react-native';
import { colors, typography } from '../../utils/theme';

type TextVariant = 'h1' | 'h2' | 'h3' | 'body' | 'caption';
type TextColor = 'primary' | 'secondary' | 'error' | 'success';

interface TextProps {
  children: React.ReactNode;
  variant?: TextVariant;
  color?: TextColor;
  style?: TextStyle;
}

const Text: React.FC<TextProps> = ({
  children,
  variant = 'body',
  color = 'primary',
  style,
}) => {
  const getVariantStyle = (): TextStyle => {
    switch (variant) {
      case 'h1':
        return { ...typography.heading, fontSize: 28 };
      case 'h2':
        return { ...typography.heading, fontSize: 24 };
      case 'h3':
        return { ...typography.title, fontSize: 20 };
      case 'caption':
        return typography.caption;
      case 'body':
      default:
        return typography.body;
    }
  };

  const getColorStyle = (): TextStyle => {
    switch (color) {
      case 'primary':
        return { color: colors.text };
      case 'secondary':
        return { color: colors.textSecondary };
      case 'error':
        return { color: colors.error };
      case 'success':
        return { color: colors.success };
      default:
        return { color: colors.text };
    }
  };

  return (
    <RNText style={[getVariantStyle(), getColorStyle(), style]}>
      {children}
    </RNText>
  );
};

export const H1: React.FC<Omit<TextProps, 'variant'>> = (props) => (
  <Text variant="h1" {...props} />
);

export const H2: React.FC<Omit<TextProps, 'variant'>> = (props) => (
  <Text variant="h2" {...props} />
);

export const H3: React.FC<Omit<TextProps, 'variant'>> = (props) => (
  <Text variant="h3" {...props} />
);

export const Body: React.FC<Omit<TextProps, 'variant'>> = (props) => (
  <Text variant="body" {...props} />
);

export const Caption: React.FC<Omit<TextProps, 'variant'>> = (props) => (
  <Text variant="caption" {...props} />
);

const styles = StyleSheet.create({});

export default Text;
export { TextProps, TextVariant, TextColor };
